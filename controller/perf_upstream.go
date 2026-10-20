package controller

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
)

var perfUpstreamClient = &http.Client{
	Timeout: 10 * time.Second,
	// Never forward the configured credential to a redirect target.
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

// proxyUpstreamPerf reads performance data without importing it into local
// counters. One configured upstream can serve multiple locally renamed models.
func proxyUpstreamPerf(c *gin.Context, summary bool, hours int) bool {
	base := strings.TrimSpace(os.Getenv("PERF_UPSTREAM_URL"))
	if base == "" {
		return false
	}
	fail := func(status int, message string) bool {
		c.JSON(status, gin.H{"success": false, "message": message})
		return true
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fail(http.StatusServiceUnavailable, "Invalid upstream performance configuration")
	}
	modelMap := map[string]string{}
	if raw := os.Getenv("PERF_UPSTREAM_MODEL_MAP"); raw != "" {
		if common.UnmarshalJsonStr(raw, &modelMap) != nil || modelMap == nil {
			return fail(http.StatusServiceUnavailable, "Invalid upstream model mapping")
		}
	}
	for local, origin := range modelMap {
		if strings.TrimSpace(local) == "" || strings.TrimSpace(origin) == "" {
			return fail(http.StatusServiceUnavailable, "Invalid upstream model mapping")
		}
	}
	if hours < 1 || hours > 168 {
		hours = 24
	}
	params := url.Values{"hours": {strconv.Itoa(hours)}}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/perf-metrics"
	localName := c.Query("model")
	if summary {
		u.Path += "/summary"
		for _, local := range c.QueryArray("recent_model") {
			origin, mapped := modelMap[local]
			if !mapped && len(modelMap) > 0 {
				continue
			}
			if !mapped {
				origin = local
			}
			params.Add("recent_model", origin)
		}
	} else {
		origin, mapped := modelMap[localName]
		if !mapped && len(modelMap) > 0 {
			return fail(http.StatusNotFound, "No upstream performance mapping for this model")
		}
		if !mapped {
			origin = localName
		}
		params.Set("model", origin)
		if group := c.Query("group"); group != "" {
			params.Set("group", group)
		}
	}
	u.RawQuery = params.Encode()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fail(http.StatusServiceUnavailable, "Invalid upstream performance configuration")
	}
	req.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(os.Getenv("PERF_UPSTREAM_API_KEY")); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	response, err := perfUpstreamClient.Do(req)
	if err != nil {
		return fail(http.StatusBadGateway, "Upstream performance is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fail(http.StatusBadGateway, "Upstream performance is unavailable")
	}
	const maxPayload = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPayload+1))
	if err != nil || len(body) > maxPayload {
		return fail(http.StatusBadGateway, "Invalid upstream performance response")
	}
	if summary {
		var payload struct {
			Success bool                          `json:"success"`
			Data    *perfmetrics.SummaryAllResult `json:"data"`
		}
		if common.Unmarshal(body, &payload) != nil || !payload.Success || payload.Data == nil {
			return fail(http.StatusBadGateway, "Invalid upstream performance response")
		}
		if len(modelMap) > 0 {
			mappedModels := make([]perfmetrics.ModelSummary, 0)
			mappedSeries := make(map[string][]perfmetrics.SuccessRatePoint)
			for local, origin := range modelMap {
				for _, model := range payload.Data.Models {
					if model.ModelName == origin {
						model.ModelName = local
						mappedModels = append(mappedModels, model)
					}
				}
				if series, ok := payload.Data.RecentSuccessSeries[origin]; ok {
					mappedSeries[local] = series
				}
			}
			payload.Data.Models = mappedModels
			payload.Data.RecentSuccessSeries = mappedSeries
			// The upstream aggregate may include models outside the mapping.
			payload.Data.Summary = nil
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": payload.Data, "source": "upstream"})
	} else {
		var payload struct {
			Success bool                     `json:"success"`
			Data    *perfmetrics.QueryResult `json:"data"`
		}
		if common.Unmarshal(body, &payload) != nil || !payload.Success || payload.Data == nil || payload.Data.ModelName != params.Get("model") {
			return fail(http.StatusBadGateway, "Invalid upstream performance response")
		}
		payload.Data.ModelName = localName
		c.JSON(http.StatusOK, gin.H{"success": true, "data": payload.Data, "source": "upstream"})
	}
	return true
}
