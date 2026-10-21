package controller

import (
	"context"
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

// perfUpstreamTimeout bounds every upstream request. It is a constant because
// the client Timeout is swapped for httptest clients in tests.
const perfUpstreamTimeout = 10 * time.Second

// perfHouseTimeout bounds one house inside a multi-house summary so a dead
// house cannot hold the whole pricing page for the full client timeout.
const perfHouseTimeout = 3 * time.Second

var perfUpstreamClient = &http.Client{
	Timeout: perfUpstreamTimeout,
	// Never forward the configured credential to a redirect target.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return http.ErrUseLastResponse
		}
		first := via[0]
		if first.URL.Hostname() != req.URL.Hostname() {
			return http.ErrUseLastResponse
		}
		if first.Header.Get("Authorization") != "" && req.Header.Get("Authorization") == first.Header.Get("Authorization") {
			req.Header.Del("Authorization")
		}
		if len(via) >= 3 {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// proxyUpstreamPerf reads performance data without importing it into local
// counters. One configured upstream can serve multiple locally renamed models.
func proxyUpstreamPerf(c *gin.Context, summary bool, hours int) bool {
	houses, errMsg := parsePerfUpstreamHouses()
	if errMsg != "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": errMsg})
		return true
	}
	if len(houses) == 0 {
		return false
	}
	fail := func(status int, message string) bool {
		c.JSON(status, gin.H{"success": false, "message": message})
		return true
	}
	explicitMap := map[string]string{}
	if raw := os.Getenv("PERF_UPSTREAM_MODEL_MAP"); raw != "" {
		if common.UnmarshalJsonStr(raw, &explicitMap) != nil || explicitMap == nil {
			return fail(http.StatusServiceUnavailable, "Invalid upstream model mapping")
		}
	}
	for local, origin := range explicitMap {
		if strings.TrimSpace(local) == "" || strings.TrimSpace(origin) == "" {
			return fail(http.StatusServiceUnavailable, "Invalid upstream model mapping")
		}
	}
	if hours < 1 || hours > 168 {
		hours = 24
	}
	localName := c.Query("model")
	restricted := len(explicitMap) > 0
	if summary {
		if len(houses) == 1 {
			body, status := perfSummaryFor(c.Request.Context(), houses[0], hours)
			if status != http.StatusOK {
				return fail(status, "Upstream performance is unavailable")
			}
			if !writeSingleHouseSummary(c, houses[0], body, explicitMap, restricted) {
				return true
			}
			return true
		}
		writeMultiHouseSummary(c, houses, explicitMap, hours)
		return true
	}
	house, ok := houseForLocal(localName, houses)
	if !ok {
		return fail(http.StatusNotFound, "No upstream performance mapping for this model")
	}
	if strings.TrimSpace(localName) == "" {
		return fail(http.StatusNotFound, "No upstream performance mapping for this model")
	}
	group := c.Query("group")
	var payload struct {
		Success bool                     `json:"success"`
		Data    *perfmetrics.QueryResult `json:"data"`
	}
	matched := false
	for _, candidate := range perfDetailCandidates(c.Request.Context(), house, localName, explicitMap) {
		query := url.Values{"hours": {strconv.Itoa(hours)}, "model": {candidate}}
		if group != "" {
			query.Set("group", group)
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), perfUpstreamTimeout)
		body, status := fetchUpstreamPerf(ctx, house, "/api/perf-metrics", query)
		cancel()
		if status != http.StatusOK {
			continue
		}
		var attempt struct {
			Success bool                     `json:"success"`
			Data    *perfmetrics.QueryResult `json:"data"`
		}
		// A house that answers with another name is answering another model.
		if common.Unmarshal(body, &attempt) != nil || !attempt.Success || attempt.Data == nil || attempt.Data.ModelName != candidate {
			continue
		}
		payload = attempt
		matched = true
		break
	}
	if !matched {
		return fail(http.StatusBadGateway, "Upstream performance is unavailable")
	}
	payload.Data.ModelName = localName
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload.Data, "source": "upstream"})
	return true
}

func requestedLocalNames(c *gin.Context, explicitMap map[string]string) []string {
	// The pricing page names the models it shows; only fall back to the whole
	// configured map when a caller asks for the summary without a model list.
	if recent := c.QueryArray("recent_model"); len(recent) > 0 {
		return recent
	}
	locals := make([]string, 0, len(explicitMap))
	for local := range explicitMap {
		locals = append(locals, local)
	}
	return locals
}

// perfDetailCandidates lists the upstream names worth asking for, most likely
// first: the configured alias, then the closest name the house actually
// reports, then the decorated/bare guesses.
func perfDetailCandidates(ctx context.Context, house perfUpstreamHouse, local string, explicitMap map[string]string) []string {
	out := make([]string, 0, 4)
	seen := make(map[string]bool, 4)
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if explicit, ok := explicitMap[local]; ok {
		add(explicit)
	}
	// The index comes from the summary the cards already requested; a cold
	// cache only falls back to the decorated/bare guesses below.
	if index := perfCachedHouseIndex(house.Code); index != nil {
		if origin, ok := index.resolve(local, explicitMap); ok {
			add(origin)
		}
	}
	if guess, ok := resolveUpstreamPerfName(local, nil); ok {
		add(guess)
	}
	return out
}

func perfSummaryNames(data *perfmetrics.SummaryAllResult) []string {
	names := make([]string, 0, len(data.Models)+len(data.RecentSuccessSeries))
	for _, model := range data.Models {
		names = append(names, model.ModelName)
	}
	for name := range data.RecentSuccessSeries {
		names = append(names, name)
	}
	return names
}

// resolveUpstreamPerfName maps a shop-facing name onto an upstream stats name.
// Exact env entries win. Otherwise strip the local shop prefix/price and accept
// only "[tag]stem" or the bare stem. Never invent a different tag house.
func resolveUpstreamPerfName(local string, explicitMap map[string]string) (string, bool) {
	if origin, ok := explicitMap[local]; ok {
		return origin, true
	}
	stem, tagOK, tag := stripLocalPerfName(local)
	if stem == "" {
		return "", false
	}
	if tagOK && tag != "" {
		return "[" + tag + "]" + stem, true
	}
	return stem, true
}

func stripLocalPerfName(name string) (stem string, hasTag bool, tag string) {
	s := strings.TrimSpace(name)
	if s == "" {
		return "", false, ""
	}
	if after, ok := strings.CutPrefix(s, "[DM]-"); ok {
		s = after
	} else if len(s) > 0 && s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end > 1 && end+1 < len(s) && s[end+1] == '-' {
			s = s[end+2:]
		}
	}
	if len(s) > 0 && s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end > 1 {
			channel := s[1:end]
			if (channel == "AG" || channel == "CLI") && end+1 < len(s) && s[end+1] == '-' {
				s = s[end+2:]
			}
		}
	}
	if dash := strings.LastIndexByte(s, '-'); dash >= 0 && dash+2 < len(s) && s[dash+1] == '[' && strings.HasSuffix(s, "]") {
		price := s[dash+2 : len(s)-1]
		if isDecimalPrice(price) {
			s = s[:dash]
		}
	}
	if len(s) > 0 && s[0] == '[' {
		end := strings.IndexByte(s, ']')
		if end > 1 && end+1 < len(s) && s[end+1] == '-' {
			return s[end+2:], true, s[1:end]
		}
	}
	return s, false, ""
}

func isDecimalPrice(value string) bool {
	if value == "" {
		return false
	}
	dot := false
	digit := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch == '.' {
			if dot {
				return false
			}
			dot = true
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
		digit = true
	}
	return digit
}
