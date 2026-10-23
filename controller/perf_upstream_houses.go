package controller

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
)

type perfUpstreamHouse struct {
	Code string
	URL  string
	Key  string
}

func parsePerfUpstreamHouses() ([]perfUpstreamHouse, string) {
	if raw := strings.TrimSpace(os.Getenv("PERF_UPSTREAMS")); raw != "" {
		parsed := map[string]string{}
		if common.UnmarshalJsonStr(raw, &parsed) != nil || parsed == nil {
			return nil, "Invalid upstream performance configuration"
		}
		houses := make([]perfUpstreamHouse, 0, len(parsed))
		for code, base := range parsed {
			house, errMsg := newPerfUpstreamHouse(code, base, os.Getenv("PERF_UPSTREAM_API_KEY"))
			if errMsg != "" {
				return nil, errMsg
			}
			houses = append(houses, house)
		}
		if len(houses) == 0 {
			return nil, "Invalid upstream performance configuration"
		}
		return houses, ""
	}
	base := strings.TrimSpace(os.Getenv("PERF_UPSTREAM_URL"))
	if base == "" {
		return nil, ""
	}
	house, errMsg := newPerfUpstreamHouse("DM", base, os.Getenv("PERF_UPSTREAM_API_KEY"))
	if errMsg != "" {
		return nil, errMsg
	}
	return []perfUpstreamHouse{house}, ""
}

func newPerfUpstreamHouse(code, base, key string) (perfUpstreamHouse, string) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return perfUpstreamHouse{}, "Invalid upstream performance configuration"
	}
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return perfUpstreamHouse{}, "Invalid upstream performance configuration"
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return perfUpstreamHouse{}, "Invalid upstream performance configuration"
	}
	return perfUpstreamHouse{Code: code, URL: strings.TrimRight(strings.TrimSpace(base), "/"), Key: strings.TrimSpace(key)}, ""
}

func localShopCode(name string) string {
	s := strings.TrimSpace(name)
	if len(s) < 4 || s[0] != '[' {
		return ""
	}
	end := strings.IndexByte(s, ']')
	if end < 2 {
		return ""
	}
	return strings.ToUpper(s[1:end])
}

func houseForLocal(local string, houses []perfUpstreamHouse) (perfUpstreamHouse, bool) {
	code := localShopCode(local)
	if code == "" {
		if len(houses) == 1 {
			return houses[0], true
		}
		return perfUpstreamHouse{}, false
	}
	for _, house := range houses {
		if house.Code == code {
			return house, true
		}
	}
	return perfUpstreamHouse{}, false
}

func perfSummaryCacheKey(house perfUpstreamHouse, hours int) string {
	return house.Code + "|" + house.URL + "|" + strconv.Itoa(hours)
}

// perfRefreshInterval keeps every house summary warm so a pricing page view
// never waits for an upstream to answer.
const perfRefreshInterval = 60 * time.Second

var perfRefresherOnce sync.Once

// StartPerfUpstreamRefresher mirrors each house summary into the cache on a
// timer. It is safe to call more than once.
func StartPerfUpstreamRefresher() {
	perfRefresherOnce.Do(func() {
		go func() {
			refreshPerfUpstreamHouses()
			ticker := time.NewTicker(perfRefreshInterval)
			defer ticker.Stop()
			for range ticker.C {
				refreshPerfUpstreamHouses()
			}
		}()
	})
}

// perfSummaryFor returns a house summary from the warm cache, fetching only
// when this house has nothing cached and is not known to be down.
func perfSummaryFor(ctx context.Context, house perfUpstreamHouse, hours int) ([]byte, int) {
	key := perfSummaryCacheKey(house, hours)
	if body, ok := perfCachedSummary(key); ok {
		return body, http.StatusOK
	}
	if perfHouseDown(house.Code) {
		return nil, http.StatusServiceUnavailable
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, perfHouseTimeout)
	defer cancel()
	body, status := fetchUpstreamPerf(timeoutCtx, house, "/api/perf-metrics/summary", url.Values{"hours": {strconv.Itoa(hours)}})
	if status != http.StatusOK {
		perfMarkHouseDown(house.Code)
		return nil, status
	}
	perfMarkHouseUp(house.Code)
	perfStoreSummary(key, body)
	return body, http.StatusOK
}

func refreshPerfUpstreamHouses() {
	houses, errMsg := parsePerfUpstreamHouses()
	if errMsg != "" || len(houses) == 0 {
		return
	}
	const hours = 24
	params := url.Values{"hours": {strconv.Itoa(hours)}}
	var wg sync.WaitGroup
	for _, house := range houses {
		house := house
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), perfHouseTimeout)
			defer cancel()
			body, status := fetchUpstreamPerf(ctx, house, "/api/perf-metrics/summary", params)
			if status != http.StatusOK {
				perfMarkHouseDown(house.Code)
				return
			}
			perfMarkHouseUp(house.Code)
			perfStoreSummary(perfSummaryCacheKey(house, hours), body)
			var payload struct {
				Success bool                          `json:"success"`
				Data    *perfmetrics.SummaryAllResult `json:"data"`
			}
			if common.Unmarshal(body, &payload) == nil && payload.Success && payload.Data != nil {
				perfStoreHouseIndex(house.Code, newPerfNameIndex(perfSummaryNames(payload.Data)))
			}
		}()
	}
	wg.Wait()
}

func fetchUpstreamPerf(ctx context.Context, house perfUpstreamHouse, path string, params url.Values) ([]byte, int) {
	target, err := url.Parse(house.URL)
	if err != nil {
		return nil, http.StatusServiceUnavailable
	}
	target.Path = strings.TrimRight(target.Path, "/") + path
	target.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, http.StatusServiceUnavailable
	}
	req.Header.Set("Accept", "application/json")
	if house.Key != "" {
		req.Header.Set("Authorization", "Bearer "+house.Key)
	}
	response, err := perfUpstreamClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, http.StatusBadGateway
	}
	const maxPayload = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPayload+1))
	if err != nil || len(body) > maxPayload {
		return nil, http.StatusBadGateway
	}
	return body, http.StatusOK
}

func writeSingleHouseSummary(c *gin.Context, house perfUpstreamHouse, body []byte, explicitMap map[string]string, restricted bool) bool {
	var payload struct {
		Success bool                          `json:"success"`
		Data    *perfmetrics.SummaryAllResult `json:"data"`
	}
	if common.Unmarshal(body, &payload) != nil || !payload.Success || payload.Data == nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "Invalid upstream performance response"})
		return false
	}
	perfStoreHouseIndex(house.Code, newPerfNameIndex(perfSummaryNames(payload.Data)))
	if restricted {
		index := newPerfNameIndex(perfSummaryNames(payload.Data))
		upstreamByOrigin := make(map[string]perfmetrics.ModelSummary, len(payload.Data.Models))
		for _, model := range payload.Data.Models {
			upstreamByOrigin[model.ModelName] = model
		}
		mappedModels := make([]perfmetrics.ModelSummary, 0)
		mappedSeries := make(map[string][]perfmetrics.SuccessRatePoint)
		locals := requestedLocalNames(c, explicitMap)
		seenLocal := map[string]bool{}
		for _, local := range locals {
			if seenLocal[local] {
				continue
			}
			seenLocal[local] = true
			origin, ok := index.resolve(local, explicitMap)
			if !ok {
				continue
			}
			model, found := upstreamByOrigin[origin]
			if !found {
				continue
			}
			model.ModelName = local
			mappedModels = append(mappedModels, model)
			if series, ok := payload.Data.RecentSuccessSeries[origin]; ok {
				mappedSeries[local] = series
			}
		}
		payload.Data.Models = mappedModels
		payload.Data.RecentSuccessSeries = mappedSeries
		payload.Data.Summary = nil
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload.Data, "source": "upstream"})
	return true
}

type houseSummary struct {
	house   perfUpstreamHouse
	payload *perfmetrics.SummaryAllResult
	index   *perfNameIndex
}

func writeMultiHouseSummary(c *gin.Context, houses []perfUpstreamHouse, explicitMap map[string]string, hours int) {
	wanted := requestedLocalNames(c, explicitMap)
	needed := map[string]perfUpstreamHouse{}
	for _, local := range wanted {
		if house, ok := houseForLocal(local, houses); ok {
			needed[house.Code] = house
		}
	}
	if len(needed) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"source":  "upstream",
			"data":    &perfmetrics.SummaryAllResult{},
		})
		return
	}
	collected := make([]houseSummary, 0, len(needed))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, house := range needed {
		house := house
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, status := perfSummaryFor(c.Request.Context(), house, hours)
			if status != http.StatusOK {
				return
			}
			var payload struct {
				Success bool                          `json:"success"`
				Data    *perfmetrics.SummaryAllResult `json:"data"`
			}
			if common.Unmarshal(body, &payload) != nil || !payload.Success || payload.Data == nil {
				return
			}
			index := newPerfNameIndex(perfSummaryNames(payload.Data))
			perfStoreHouseIndex(house.Code, index)
			mu.Lock()
			collected = append(collected, houseSummary{house: house, payload: payload.Data, index: index})
			mu.Unlock()
		}()
	}
	wg.Wait()

	byCode := map[string]houseSummary{}
	for _, item := range collected {
		byCode[item.house.Code] = item
	}
	mappedModels := make([]perfmetrics.ModelSummary, 0)
	mappedSeries := make(map[string][]perfmetrics.SuccessRatePoint)
	seenLocal := map[string]bool{}
	for _, local := range wanted {
		if seenLocal[local] {
			continue
		}
		seenLocal[local] = true
		house, ok := houseForLocal(local, houses)
		if !ok {
			continue
		}
		item, ok := byCode[house.Code]
		if !ok {
			continue
		}
		upstreamByOrigin := make(map[string]perfmetrics.ModelSummary, len(item.payload.Models))
		for _, model := range item.payload.Models {
			upstreamByOrigin[model.ModelName] = model
		}
		origin, ok := item.index.resolve(local, explicitMap)
		if !ok {
			continue
		}
		model, found := upstreamByOrigin[origin]
		if !found {
			continue
		}
		model.ModelName = local
		mappedModels = append(mappedModels, model)
		if series, ok := item.payload.RecentSuccessSeries[origin]; ok {
			mappedSeries[local] = series
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"source":  "upstream",
		"data": &perfmetrics.SummaryAllResult{
			Models:              mappedModels,
			RecentSuccessSeries: mappedSeries,
		},
	})
}
