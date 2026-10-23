package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerfParseNameStripsShopAndUpstreamDecoration(t *testing.T) {
	for _, tc := range []struct{ name, core string }{
		{"[DZ]-[AG]-[Call]-gemini-3.8-flash-128-nothinking-[0.21]", "gemini-3.8-flash-128-nothinking"},
		{"[BT]-[AG]-【-0.03】gemini-3.8-flash-high-[0.31]", "gemini-3.8-flash-high"},
		{"[KFC]-[AG]-step-5-preview-[0]", "step-5-preview"},
		{"[按次]gemini-3.8-flash", "gemini-3.8-flash"},
		{"[反重力][自营2][量] claude-opus-4-6-thinking", "claude-opus-4-6-thinking"},
		{"claude-haiku-4-5-20251001[次-1]", "claude-haiku-4-5-20251001"},
		{"[N][破限]-gemini-2.5-pro（4）", "gemini-2.5-pro"},
		{"[MI]-[AG]-[AntiC]-/gemini-3.7-flash-high-[0.16]", "gemini-3.7-flash-high"},
		{"gemini-3.8-flash", "gemini-3.8-flash"},
	} {
		assert.Equal(t, tc.core, perfParseName(tc.name).Core, tc.name)
	}
}

func TestPerfNameIndexResolvesPerHouseDecorations(t *testing.T) {
	for _, tc := range []struct {
		house string
		known []string
		local string
		want  string
	}{
		{
			house: "DZ",
			known: []string{"[按次]gemini-3.8-flash", "[按次]gemini-3.8-flash-128", "[按量]deepseek-v4-pro-thinking"},
			local: "[DZ]-[AG]-[Call]-gemini-3.8-flash-128-nothinking-[0.21]",
			want:  "[按次]gemini-3.8-flash-128",
		},
		{
			house: "DZ",
			known: []string{"[按次]gemini-3.8-flash", "[按量]deepseek-v4-pro-thinking"},
			local: "[DZ]-[AG]-[Call]-deepseek-v4-pro-thinking-[0.62]",
			want:  "[按量]deepseek-v4-pro-thinking",
		},
		{
			house: "SAKE",
			known: []string{"[应急]gemini-3-pro-preview-searching[满血-5]", "[应急]gemini-3-pro-preview[满血-5]", "[应急]gemini-3-pro-preview-thinking[满血-5]"},
			local: "[SAKE]-[AG]-[Em]-gemini-3-pro-preview-maxthinking-[0.47]",
			want:  "[应急]gemini-3-pro-preview[满血-5]",
		},
		{
			house: "KFC",
			known: []string{"[反重力][自营2][量] claude-opus-4-6-thinking", "[Kiro] claude-opus-4-6-thinking [不补]", "[aws]minimax-m2"},
			local: "[KFC]-[AG]-[AGX3]-claude-opus-4-6-thinking-[2.14]",
			want:  "[反重力][自营2][量] claude-opus-4-6-thinking",
		},
		{
			house: "MI",
			known: []string{"deepseek-v4-flash-0731", "gemini-4-argon"},
			local: "[MI]-[AG]-[XJ]-deepseek-ai/deepseek-v4-flash-0731-[0.2]",
			want:  "deepseek-v4-flash-0731",
		},
		{
			house: "LHAO",
			known: []string{"[企业E]gemini-4-argon", "[满血]gemini-3.8-flash"},
			local: "[LHAO]-[AG]-[FullAis]-gemini-3.8-flash-[0.83]",
			want:  "[满血]gemini-3.8-flash",
		},
	} {
		index := newPerfNameIndex(tc.known)
		origin, ok := index.resolve(tc.local, nil)
		require.True(t, ok, tc.local)
		assert.Equal(t, tc.want, origin, tc.local)
	}
}

func TestPerfNameIndexDoesNotMergeVersions(t *testing.T) {
	index := newPerfNameIndex([]string{"[aws]minimax-m2", "gpt-image-2.5-flare"})
	_, ok := index.resolve("[KFC]-[AG]-[aws]-minimax-m2.1-[0.09]", nil)
	assert.False(t, ok)
	_, ok = index.resolve("[DM]-[AG]-[Off]-gpt-image-2-[2.55]", nil)
	assert.False(t, ok)
}

func TestPerfNameIndexPrefersConfiguredAlias(t *testing.T) {
	index := newPerfNameIndex([]string{"[官逆]gemini-3.5-flash", "[按次]gemini-3.5-flash"})
	explicit := map[string]string{"[DM]-[AG]-[OffRev]-gemini-3.5-flash-[0.1]": "[官逆]gemini-3.5-flash"}
	origin, ok := index.resolve("[DM]-[AG]-[OffRev]-gemini-3.5-flash-[0.1]", explicit)
	require.True(t, ok)
	assert.Equal(t, "[官逆]gemini-3.5-flash", origin)
}

func TestPerfNameIndexMatchesChannelPrefixedCores(t *testing.T) {
	index := newPerfNameIndex([]string{"[特价kiro量]claude-sonnet-5", "claude-sonnet-5-5", "claude-sonnet-5"})
	origin, ok := index.resolve("[DM]-[CLI]-[kiro2]-kiro-claude-sonnet-5-[3.57]", nil)
	require.True(t, ok)
	// The same core is listed on several channels; any of them answers for it.
	assert.Equal(t, "claude-sonnet-5", perfParseName(origin).Core)
}

func TestPerfNameIndexPrefersSubsetOverExtraVersion(t *testing.T) {
	index := newPerfNameIndex([]string{"gemini-4-argon-low", "gemini-4-argon-low-thinking"})
	origin, ok := index.resolve("[MI]-[AG]-[UltraC]-gemini-4-argon-low-[3.04]", nil)
	require.True(t, ok)
	assert.Equal(t, "gemini-4-argon-low", origin)
}

func TestPerfNameIndexMatchesUnseparatedCores(t *testing.T) {
	index := newPerfNameIndex([]string{"gemini-Gemma 4"})
	origin, ok := index.resolve("[SAKE]-[AG]-gemini-Gemma4-[0.09]", nil)
	require.True(t, ok)
	assert.Equal(t, "gemini-Gemma 4", origin)
}

func TestPerfNameIndexTrimsLeadingSeparators(t *testing.T) {
	index := newPerfNameIndex([]string{"gemini-3.7-flash-high"})
	origin, ok := index.resolve("[MI]-[AG]-[AntiC]-/gemini-3.7-flash-high-[0.16]", nil)
	require.True(t, ok)
	assert.Equal(t, "gemini-3.7-flash-high", origin)
}
