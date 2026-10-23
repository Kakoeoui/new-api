package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUptimeStatusRequiresUserAuth(t *testing.T) {
	assertAnonymousRouteUnauthorized(t, "/api/uptime/status", "monitors")
}

func TestPricingRequiresUserAuthByDefault(t *testing.T) {
	assertAnonymousRouteUnauthorized(t, "/api/pricing", `"model"`)
}

func TestPerfMetricsRequireUserAuthByDefault(t *testing.T) {
	assertAnonymousRouteUnauthorized(t, "/api/perf-metrics/summary", "models")
	assertAnonymousRouteUnauthorized(t, "/api/perf-metrics?model=gpt-4", "buckets")
}

func assertAnonymousRouteUnauthorized(t *testing.T, path, leakedField string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code, path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotContains(t, body, "data", path)
	require.NotContains(t, recorder.Body.String(), leakedField, path)
}
