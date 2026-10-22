package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyUpstreamPerf(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		summary  bool
		query    string
		mapping  string
		response string
		status   int
	}{
		{
			name: "summary maps aliases and hides unmapped models", summary: true,
			query: "?recent_model=local", mapping: `{"local":"origin","alias":"origin"}`,
			response: `{"success":true,"data":{"models":[{"model_name":"origin","success_rate":99},{"model_name":"private"}],"recent_success_series":{"origin":[{"ts":123,"success_rate":99}]}}}`,
			status:   http.StatusOK,
		},
		{
			name: "detail maps the requested name", query: "?model=local", mapping: `{"local":"origin"}`,
			response: `{"success":true,"data":{"model_name":"origin","groups":[],"summary":{"success_rate":99}}}`,
			status:   http.StatusOK,
		},
		{
			name: "invalid upstream JSON is not passed through", summary: true,
			response: `<html>secret-token</html>`, status: http.StatusBadGateway,
		},
		{
			name: "upstream business error is not exposed", summary: true,
			response: `{"success":false,"message":"secret-token"}`, status: http.StatusBadGateway,
		},
		{
			name: "missing data is rejected", summary: true,
			response: `{"success":true}`, status: http.StatusBadGateway,
		},
		{
			name: "mismatched detail is rejected", query: "?model=local",
			response: `{"success":true,"data":{"model_name":"wrong"}}`, status: http.StatusBadGateway,
		},
		{
			name: "summary infers stripped shop names onto a bare upstream model", summary: true,
			query:   "?recent_model=%5BDM%5D-%5BAG%5D-%5Banti%5D-gemini-3.8-flash-%5B0.1%5D",
			mapping: `{"kept":"kept-origin"}`,
			response: `{"success":true,"data":{"models":[{"model_name":"gemini-3.8-flash","success_rate":88},{"model_name":"[anti4]gemini-3.8-flash-high","success_rate":11}],"recent_success_series":{"gemini-3.8-flash":[{"ts":1,"success_rate":88}]}}}`,
			status: http.StatusOK,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
				assert.Empty(t, r.Header.Get("Cookie"))
				assert.Equal(t, "24", r.URL.Query().Get("hours"))
				// empty mapping uses the requested name as-is
				if tc.mapping != "" && !tc.summary {
						assert.Equal(t, "origin", r.URL.Query().Get("model"))
					}
				_, err := io.WriteString(w, tc.response)
				assert.NoError(t, err)
			}))
			defer server.Close()
			previousClient := perfUpstreamClient
			perfUpstreamClient = server.Client()
			t.Cleanup(func() { perfUpstreamClient = previousClient })
			t.Setenv("PERF_UPSTREAM_URL", server.URL)
			t.Setenv("PERF_UPSTREAM_API_KEY", "secret-token")
			t.Setenv("PERF_UPSTREAM_MODEL_MAP", tc.mapping)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics"+tc.query, nil)
			c.Request.Header.Set("Cookie", "local-session=private")
			require.True(t, proxyUpstreamPerf(c, tc.summary, 24))
			assert.Equal(t, tc.status, w.Code)
			assert.NotContains(t, w.Body.String(), "secret-token")
			if tc.status == http.StatusOK {
				var payload map[string]any
				require.NoError(t, common.Unmarshal(w.Body.Bytes(), &payload))
				assert.Equal(t, "upstream", payload["source"])
				assert.NotContains(t, w.Body.String(), "private")
				if strings.Contains(tc.query, "recent_model=%5BDM%5D") {
					assert.Contains(t, w.Body.String(), "[DM]-[AG]-[anti]-gemini-3.8-flash-[0.1]")
					assert.Contains(t, w.Body.String(), `"success_rate":88`)
					assert.NotContains(t, w.Body.String(), "[anti4]")
				} else {
					assert.Contains(t, w.Body.String(), "local")
					if tc.summary {
						assert.NotContains(t, w.Body.String(), "alias")
						assert.Contains(t, w.Body.String(), `"success_rate":99`)
					}
				}
			}
		})
	}
}

func TestProxyUpstreamPerfConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		base    string
		mapping string
		query   string
		status  int
	}{
		{name: "disabled uses local metrics"},
		{name: "rejects cleartext credentials", base: "http://example.com", status: 404},
		{name: "rejects URL credentials", base: "https://user:secret@example.com", status: 503},
		{name: "rejects broken mapping", base: "https://example.com", mapping: "not-json", status: 503},
		{name: "rejects empty origin", base: "https://example.com", mapping: `{"local":""}`, status: 503},
		{name: "rejects unmapped model", base: "https://example.com", mapping: `{"local":"origin"}`, query: "?model=unknown", status: 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PERF_UPSTREAM_URL", tc.base)
			t.Setenv("PERF_UPSTREAM_MODEL_MAP", tc.mapping)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics"+tc.query, nil)
			handled := proxyUpstreamPerf(c, false, 24)
			assert.Equal(t, tc.base != "", handled)
			if handled {
				assert.Equal(t, tc.status, w.Code)
				assert.False(t, strings.Contains(w.Body.String(), "secret"))
			}
		})
	}
}

func TestPerfUpstreamDoesNotFollowRedirects(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/credential-leak" {
			t.Fatal("redirect followed")
		}
		http.Redirect(w, r, "https://example.com/credential-leak", http.StatusFound)
	}))
	defer server.Close()
	previousClient := perfUpstreamClient
	client := *previousClient
	client.Transport = server.Client().Transport
	perfUpstreamClient = &client
	t.Cleanup(func() { perfUpstreamClient = previousClient })
	t.Setenv("PERF_UPSTREAM_URL", server.URL)
	t.Setenv("PERF_UPSTREAM_MODEL_MAP", "")
	t.Setenv("PERF_UPSTREAM_API_KEY", "secret-token")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics/summary", nil)
	require.True(t, proxyUpstreamPerf(c, true, 24))
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.NotContains(t, w.Body.String(), "secret-token")
}

func TestParsePerfUpstreamHouses(t *testing.T) {
	t.Setenv("PERF_UPSTREAMS", "")
	t.Setenv("PERF_UPSTREAM_URL", "https://api.chenmoai.com")
	houses, errMsg := parsePerfUpstreamHouses()
	require.Empty(t, errMsg)
	require.Len(t, houses, 1)
	assert.Equal(t, "DM", houses[0].Code)
	assert.Equal(t, "https://api.chenmoai.com", houses[0].URL)
}

func TestHouseForLocal(t *testing.T) {
	houses := []perfUpstreamHouse{{Code: "DM", URL: "https://api.chenmoai.com"}, {Code: "GM", URL: "https://api.gemai.cc"}}
	house, ok := houseForLocal("[GM]-[AG]-gemini-3.8-flash-[0.1]", houses)
	assert.True(t, ok)
	assert.Equal(t, "GM", house.Code)
	_, ok = houseForLocal("[BT]-[AG]-model-[0.1]", houses)
	assert.False(t, ok)
}

func TestProxyUpstreamPerfRoutesHouses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var hits []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Host)
		_, _ = io.WriteString(w, `{"success":true,"data":{"models":[{"model_name":"gemini-3.8-flash","success_rate":77}],"recent_success_series":{"gemini-3.8-flash":[{"ts":1,"success_rate":77}]}}}`)
	}))
	defer server.Close()
	previous := perfUpstreamClient
	perfUpstreamClient = server.Client()
	t.Cleanup(func() { perfUpstreamClient = previous })
	t.Setenv("PERF_UPSTREAMS", `{"DM":"`+server.URL+`","GM":"`+server.URL+`"}`)
	t.Setenv("PERF_UPSTREAM_MODEL_MAP", "")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/perf-metrics/summary?recent_model="+url.QueryEscape("[GM]-[AG]-gemini-3.8-flash-[0.1]")+"&recent_model="+url.QueryEscape("[BT]-[AG]-hidden-[0.1]"), nil)
	require.True(t, proxyUpstreamPerf(c, true, 24))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[GM]-[AG]-gemini-3.8-flash-[0.1]")
	assert.Contains(t, w.Body.String(), `"success_rate":77`)
	assert.NotContains(t, w.Body.String(), "[BT]")
	assert.Equal(t, 1, len(hits))
}

func TestResolveUpstreamPerfName(t *testing.T) {
	explicit := map[string]string{
		"[DM]-[AG]-[OffRev]-gemini-3.5-flash-[0.1]": "[官逆]gemini-3.5-flash",
	}
	origin, ok := resolveUpstreamPerfName("[DM]-[AG]-[OffRev]-gemini-3.5-flash-[0.1]", explicit)
	assert.True(t, ok)
	assert.Equal(t, "[官逆]gemini-3.5-flash", origin)

	origin, ok = resolveUpstreamPerfName("[DM]-[AG]-[anti]-gemini-3.8-flash-[0.1]", explicit)
	assert.True(t, ok)
	assert.Equal(t, "[anti]gemini-3.8-flash", origin)

	origin, ok = resolveUpstreamPerfName("[DM]-[AG]-gemini-3.8-flash-[0.1]", nil)
	assert.True(t, ok)
	assert.Equal(t, "gemini-3.8-flash", origin)

	origin, ok = resolveUpstreamPerfName("[DM]-[AG]-[anti4]-gemini-3.8-flash-high-[0.26]", nil)
	assert.True(t, ok)
	assert.Equal(t, "[anti4]gemini-3.8-flash-high", origin)
}
