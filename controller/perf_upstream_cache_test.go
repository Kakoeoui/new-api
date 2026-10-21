package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A house summary that is already in the cache is answered without touching
// the network, so a page view never waits for a slow upstream.
func TestProxyUpstreamPerfServesCachedHouseSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	house := perfUpstreamHouse{Code: "DZ", URL: "https://dz.example.test"}
	perfStoreSummary(perfSummaryCacheKey(house, 24), []byte(`{"success":true,"data":{"models":[`+
		`{"model_name":"[按次]gemini-3.8-flash-128","success_rate":77}],`+
		`"recent_success_series":{"[按次]gemini-3.8-flash-128":[{"ts":1,"success_rate":77}]}}}`))
	t.Setenv("PERF_UPSTREAMS", `{"DZ":"https://dz.example.test"}`)
	// A single configured house only renames rows when an alias map exists.
	t.Setenv("PERF_UPSTREAM_MODEL_MAP", `{"kept":"kept-origin"}`)

	local := "[DZ]-[AG]-[Call]-gemini-3.8-flash-128-nothinking-[0.21]"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics/summary?recent_model="+url.QueryEscape(local), nil)
	require.True(t, proxyUpstreamPerf(c, true, 24))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), local)
	assert.Contains(t, w.Body.String(), `"success_rate":77`)
}

func TestRefreshPerfUpstreamHousesWarmsCache(t *testing.T) {
	var hits int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.WriteString(w, `{"success":true,"data":{"models":[{"model_name":"gemini-3.8-flash","success_rate":55}]}}`)
	}))
	defer server.Close()
	previous := perfUpstreamClient
	perfUpstreamClient = server.Client()
	t.Cleanup(func() { perfUpstreamClient = previous })
	t.Setenv("PERF_UPSTREAMS", `{"GM":"`+server.URL+`"}`)
	t.Setenv("PERF_UPSTREAM_MODEL_MAP", "")

	refreshPerfUpstreamHouses()
	house := perfUpstreamHouse{Code: "GM", URL: server.URL}
	body, ok := perfCachedSummary(perfSummaryCacheKey(house, 24))
	require.True(t, ok)
	assert.Contains(t, string(body), "gemini-3.8-flash")
	assert.EqualValues(t, 1, atomic.LoadInt32(&hits))
}

func TestPerfCachedSummaryExpires(t *testing.T) {
	assert.False(t, perfHouseDown("NOPE"))
	perfMarkHouseDown("NOPE")
	assert.True(t, perfHouseDown("NOPE"))
	perfMarkHouseUp("NOPE")
	assert.False(t, perfHouseDown("NOPE"))
}
