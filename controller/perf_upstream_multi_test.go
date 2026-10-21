package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each house decorates the same kind of model differently, so the summary has
// to be matched per house instead of through one shared alias table.
func TestProxyUpstreamPerfResolvesDecoratedHouseNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/dz/"):
			_, _ = io.WriteString(w, `{"success":true,"data":{"models":[`+
				`{"model_name":"[按次]gemini-3.8-flash","success_rate":41},`+
				`{"model_name":"[按次]gemini-3.8-flash-128","success_rate":93}],`+
				`"recent_success_series":{"[按次]gemini-3.8-flash-128":[{"ts":1,"success_rate":93}]}}}`)
		case strings.HasPrefix(r.URL.Path, "/sake/"):
			_, _ = io.WriteString(w, `{"success":true,"data":{"models":[`+
				`{"model_name":"[应急]gemini-3-pro-preview[满血-5]","success_rate":67},`+
				`{"model_name":"[应急]gemini-3-pro-preview-thinking[满血-5]","success_rate":12}],`+
				`"recent_success_series":{"[应急]gemini-3-pro-preview[满血-5]":[{"ts":1,"success_rate":67}]}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previous := perfUpstreamClient
	perfUpstreamClient = server.Client()
	t.Cleanup(func() { perfUpstreamClient = previous })
	t.Setenv("PERF_UPSTREAMS", `{"DZ":"`+server.URL+`/dz","SAKE":"`+server.URL+`/sake"}`)
	t.Setenv("PERF_UPSTREAM_MODEL_MAP", "")

	dzLocal := "[DZ]-[AG]-[Call]-gemini-3.8-flash-128-nothinking-[0.21]"
	sakeLocal := "[SAKE]-[AG]-[Em]-gemini-3-pro-preview-maxthinking-[0.47]"
	unknownLocal := "[KFC]-[AG]-[AGX3]-claude-opus-4-6-thinking-[2.14]"
	query := "/api/perf-metrics/summary?recent_model=" + url.QueryEscape(dzLocal) +
		"&recent_model=" + url.QueryEscape(sakeLocal) +
		"&recent_model=" + url.QueryEscape(unknownLocal)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, query, nil)
	require.True(t, proxyUpstreamPerf(c, true, 24))
	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, dzLocal)
	assert.Contains(t, body, `"success_rate":93`)
	assert.Contains(t, body, sakeLocal)
	assert.Contains(t, body, `"success_rate":67`)
	assert.NotContains(t, body, "[按次]")
	assert.NotContains(t, body, "[应急]")
	assert.NotContains(t, body, unknownLocal)
	assert.Contains(t, body, "recent_success_series")
}
