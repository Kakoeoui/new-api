package controller

import (
	"strings"
	"sync"
	"time"
)

// Shop names and upstream names describe the same model with different
// decoration: shops wrap them as "[DZ]-[AG]-[Call]-gemini-3.8-flash-128-[0.2]"
// while the upstream lists them as "[按次]gemini-3.8-flash-128". The tag
// vocabulary differs per house (Call/按次, Full/满血, ACMix/AC混合, ...), so
// names are compared on the undecorated model core instead of on the tags.
type perfName struct {
	Raw    string
	Core   string
	Groups []string
}

var perfGroupOpen = map[rune]rune{
	'[': ']', '【': '】', '［': '］', '「': '」', '〈': '〉', '(': ')', '（': '）',
}

var perfGroupClose = func() map[rune]rune {
	out := make(map[rune]rune, len(perfGroupOpen))
	for open, close := range perfGroupOpen {
		out[close] = open
	}
	return out
}()

// perfParseName splits a perf model name into its tag groups and model core.
// Leading groups are the channel/route tags, trailing groups are price or
// qualifier markers; both are dropped from the core.
func perfParseName(raw string) perfName {
	s := strings.TrimSpace(raw)
	groups := make([]string, 0, 4)
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}
		open := []rune(s)[0]
		close, ok := perfGroupOpen[open]
		if !ok {
			break
		}
		end := strings.IndexRune(s, close)
		if end <= 0 {
			break
		}
		if group := strings.TrimSpace(s[len(string(open)):end]); group != "" {
			groups = append(groups, strings.ToLower(group))
		}
		s = strings.TrimLeft(strings.TrimSpace(s[end+len(string(close)):]), "- ")
	}
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}
		runes := []rune(s)
		last := runes[len(runes)-1]
		open, ok := perfGroupClose[last]
		if !ok {
			break
		}
		start := strings.LastIndex(s, string(open))
		if start < 0 {
			break
		}
		if group := strings.TrimSpace(s[start+len(string(open)) : len(s)-len(string(last))]); group != "" {
			groups = append(groups, strings.ToLower(group))
		}
		s = strings.TrimRight(strings.TrimSpace(s[:start]), "- ")
	}
	core := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '_':
			return '-'
		}
		return r
	}, strings.ToLower(s))
	// Some shops publish "/model" or "model." style names.
	return perfName{Raw: raw, Core: strings.Trim(core, "-/.:"), Groups: groups}
}

const (
	perfTierNone = iota
	perfTierAffix
	perfTierLoose
	perfTierVendor
	perfTierExact
)

// perfLooseCore drops the separators shops disagree about: "gemini-Gemma4"
// and "gemini-Gemma 4" are the same model.
func perfLooseCore(core string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', '/', '.', '_', ':', ' ':
			return -1
		}
		return r
	}, core)
}

// perfNameTier ranks a candidate core against the shop core.
func perfNameTier(local, origin string) int {
	if local == "" || origin == "" {
		return perfTierNone
	}
	if local == origin {
		return perfTierExact
	}
	if strings.HasSuffix(origin, "/"+local) || strings.HasSuffix(local, "/"+origin) ||
		strings.HasPrefix(origin, local+"/") || strings.HasPrefix(local, origin+"/") {
		return perfTierVendor
	}
	if looseLocal, looseOrigin := perfLooseCore(local), perfLooseCore(origin); looseLocal != "" && looseLocal == looseOrigin {
		return perfTierLoose
	}
	if perfDashAffix(local, origin) {
		return perfTierAffix
	}
	return perfTierNone
}

// perfDashAffix reports whether one core is a dash-boundary prefix or suffix
// of the other, e.g. "gemini-3.8-flash" against "gemini-3.8-flash-128-thinking"
// or "kiro-claude-sonnet-5" against "claude-sonnet-5". Version dots
// (gpt-image-2 vs gpt-image-2.5) are deliberately not merged.
func perfDashAffix(local, origin string) bool {
	shorter, longer := local, origin
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	if len(shorter) < 8 || len(shorter)*2 < len(longer) || !strings.Contains(shorter, "-") {
		return false
	}
	if strings.HasPrefix(longer, shorter) && longer[len(shorter)] == '-' {
		return true
	}
	if strings.HasSuffix(longer, shorter) && longer[len(longer)-len(shorter)-1] == '-' {
		return true
	}
	return false
}

// perfRank orders candidates for one shop name: better tier first, then a
// candidate whose tokens all appear in the shop name (no invented version
// suffix), then the closest length, then the most tokens.
type perfRank struct {
	tier   int
	subset int
	diff   int
	tokens int
}

func (rank perfRank) betterThan(other perfRank) bool {
	if rank.tier != other.tier {
		return rank.tier > other.tier
	}
	if rank.subset != other.subset {
		return rank.subset > other.subset
	}
	if rank.diff != other.diff {
		return rank.diff < other.diff
	}
	return rank.tokens > other.tokens
}

func perfTokenCounts(core string) map[string]int {
	out := make(map[string]int, 8)
	for _, token := range strings.Split(core, "-") {
		if token != "" {
			out[token]++
		}
	}
	return out
}

// perfIsSubset reports whether every token of candidate appears in local.
func perfIsSubset(candidate, local string) bool {
	if candidate == "" || candidate == local {
		return false
	}
	available := perfTokenCounts(local)
	for token, count := range perfTokenCounts(candidate) {
		if available[token] < count {
			return false
		}
	}
	return true
}

func perfNameRank(local, candidate perfName) (perfRank, bool) {
	tier := perfNameTier(local.Core, candidate.Core)
	if tier == perfTierNone {
		return perfRank{}, false
	}
	diff := len(local.Core) - len(candidate.Core)
	if diff < 0 {
		diff = -diff
	}
	subset := 0
	if perfIsSubset(candidate.Core, local.Core) {
		subset = 1
	}
	overlap := 0
	for _, want := range local.Groups {
		for _, have := range candidate.Groups {
			if want == have {
				overlap++
				break
			}
		}
	}
	return perfRank{
		tier:   tier*100 + overlap,
		subset: subset,
		diff:   diff,
		tokens: strings.Count(candidate.Core, "-") + 1,
	}, true
}

type perfNameIndex struct {
	names []string
	items []perfName
}

func newPerfNameIndex(names []string) *perfNameIndex {
	index := &perfNameIndex{names: make([]string, 0, len(names)), items: make([]perfName, 0, len(names))}
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		index.names = append(index.names, name)
		index.items = append(index.items, perfParseName(name))
	}
	return index
}

func (index *perfNameIndex) has(name string) bool {
	if index == nil {
		return false
	}
	for _, known := range index.names {
		if known == name {
			return true
		}
	}
	return false
}

// resolve maps a shop-facing name onto the upstream name that actually exists
// in this house's payload. Explicit env entries win, then the closest core.
func (index *perfNameIndex) resolve(local string, explicitMap map[string]string) (string, bool) {
	if index == nil {
		return "", false
	}
	if explicit, ok := explicitMap[local]; ok && explicit != "" && index.has(explicit) {
		return explicit, true
	}
	want := perfParseName(local)
	// The first two groups are the shop code and the channel, never upstream tags.
	if len(want.Groups) > 2 {
		want.Groups = want.Groups[2:]
	} else {
		want.Groups = nil
	}
	best := ""
	var bestRank perfRank
	for i := range index.items {
		rank, ok := perfNameRank(want, index.items[i])
		if !ok {
			continue
		}
		if best == "" || rank.betterThan(bestRank) {
			best, bestRank = index.names[i], rank
		}
	}
	if best != "" {
		return best, true
	}
	if explicit, ok := explicitMap[local]; ok && explicit != "" {
		return explicit, true
	}
	return "", false
}

const (
	// perfIndexTTL bounds how stale a cached house index may be.
	perfIndexTTL = 2 * time.Minute
	// perfSummaryTTL is how long a snapshotted house summary stays usable.
	// A background refresher rewrites it every refresh interval, so requests
	// normally read something a few seconds old and never wait on an upstream.
	perfSummaryTTL = 5 * time.Minute
	// perfHouseDownTTL skips a house that just failed instead of making the
	// whole pricing page wait for it again.
	perfHouseDownTTL = 60 * time.Second
)

type perfCachedPayload struct {
	at   time.Time
	body []byte
}

var perfSummaryCache sync.Map    // "CODE|hours" -> perfCachedPayload
var perfHouseDownUntil sync.Map  // house code -> time.Time

func perfCachedSummary(key string) ([]byte, bool) {
	entry, ok := perfSummaryCache.Load(key)
	if !ok {
		return nil, false
	}
	cached := entry.(perfCachedPayload)
	if time.Since(cached.at) >= perfSummaryTTL {
		return nil, false
	}
	return cached.body, true
}

func perfStoreSummary(key string, body []byte) {
	if key == "" || len(body) == 0 {
		return
	}
	perfSummaryCache.Store(key, perfCachedPayload{at: time.Now(), body: body})
}

func perfHouseDown(code string) bool {
	entry, ok := perfHouseDownUntil.Load(code)
	if !ok {
		return false
	}
	return time.Now().Before(entry.(time.Time))
}

func perfMarkHouseDown(code string) {
	perfHouseDownUntil.Store(code, time.Now().Add(perfHouseDownTTL))
}

func perfMarkHouseUp(code string) {
	perfHouseDownUntil.Delete(code)
}

type perfCachedIndex struct {
	at    time.Time
	index *perfNameIndex
}

var perfIndexCache sync.Map // house code -> perfCachedIndex

func perfCachedHouseIndex(code string) *perfNameIndex {
	if entry, ok := perfIndexCache.Load(code); ok {
		cached := entry.(perfCachedIndex)
		if time.Since(cached.at) < perfIndexTTL {
			return cached.index
		}
	}
	return nil
}

func perfStoreHouseIndex(code string, index *perfNameIndex) {
	if code == "" || index == nil {
		return
	}
	perfIndexCache.Store(code, perfCachedIndex{at: time.Now(), index: index})
}
