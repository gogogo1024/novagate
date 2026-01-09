package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/gogogo1024/novagate/services/acl/internal/acl"
	"github.com/redis/go-redis/v9"
)

var statsEnabled bool

func SetStatsEnabled(v bool) {
	statsEnabled = v
}

type statsResponse struct {
	Backend  string             `json:"backend"`
	Now      string             `json:"now"`
	Redis    *redisStats        `json:"redis,omitempty"`
	InMemory *acl.InMemoryStats `json:"in_memory,omitempty"`
}

type redisStats struct {
	Prefix   string            `json:"prefix"`
	DBSize   int64             `json:"dbsize"`
	Memory   map[string]string `json:"memory"`
	Keyspace map[string]string `json:"keyspace"`
	Stats    map[string]string `json:"stats"`
	Estimate *prefixEstimate   `json:"estimate,omitempty"`
}

type prefixEstimate struct {
	Enabled          bool    `json:"enabled"`
	BudgetMS         int64   `json:"budget_ms"`
	ScanCount        int64   `json:"scan_count"`
	ElapsedMS        int64   `json:"elapsed_ms"`
	ScannedKeys      int64   `json:"scanned_keys"`
	MatchedKeys      int64   `json:"matched_keys"`
	MatchedRatio     float64 `json:"matched_ratio"`
	EstimatedKeys    int64   `json:"estimated_keys"`
	EstimatedKeysMin int64   `json:"estimated_keys_min"`
	EstimatedKeysMax int64   `json:"estimated_keys_max"`
}

// ACLStats GET /v1/acl/stats
// Guarded by server.enable_stats.
func ACLStats(ctx context.Context, c *app.RequestContext) {
	if !statsEnabled {
		c.JSON(consts.StatusNotFound, utils.H{"error": "not found"})
		return
	}

	now := time.Now().UTC()

	// Prefer Redis stats when running on RedisStore.
	if rs, ok := store.(*acl.RedisStore); ok {
		rdb := rs.Client()
		if rdb == nil {
			c.JSON(consts.StatusServiceUnavailable, utils.H{"error": "redis client not initialized"})
			return
		}

		rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		dbsize, err := rdb.DBSize(rctx).Result()
		if err != nil {
			c.JSON(consts.StatusServiceUnavailable, utils.H{"error": err.Error()})
			return
		}

		memInfo, _ := rdb.Info(rctx, "memory").Result()
		keyInfo, _ := rdb.Info(rctx, "keyspace").Result()
		stInfo, _ := rdb.Info(rctx, "stats").Result()

		est := maybeEstimatePrefixKeys(rctx, rdb, rs.Prefix(), dbsize, c)

		c.JSON(consts.StatusOK, statsResponse{
			Backend: "redis",
			Now:     now.Format(time.RFC3339Nano),
			Redis: &redisStats{
				Prefix:   rs.Prefix(),
				DBSize:   dbsize,
				Memory:   parseRedisInfo(memInfo),
				Keyspace: parseRedisInfo(keyInfo),
				Stats:    parseRedisInfo(stInfo),
				Estimate: est,
			},
		})
		return
	}

	// In-memory fallback.
	if ims, ok := store.(*acl.InMemoryStore); ok {
		st := ims.Stats()
		c.JSON(consts.StatusOK, statsResponse{
			Backend:  "in_memory",
			Now:      now.Format(time.RFC3339Nano),
			InMemory: &st,
		})
		return
	}

	c.JSON(consts.StatusOK, statsResponse{Backend: "unknown", Now: now.Format(time.RFC3339Nano)})
}

func parseRedisInfo(info string) map[string]string {
	m := make(map[string]string)
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" {
			m[k] = v
		}
	}
	return m
}

type scanClient interface {
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

func maybeEstimatePrefixKeys(ctx context.Context, rdb scanClient, prefix string, dbsize int64, c *app.RequestContext) *prefixEstimate {
	if !queryBool(c, "estimate_prefix") {
		return nil
	}

	budgetMS := queryInt64Bounded(c, "budget_ms", 50, 10, 2000)
	scanCount := queryInt64Bounded(c, "scan_count", 1000, 10, 10000)

	scanned, matched, elapsedMS := samplePrefixKeys(ctx, rdb, prefix, time.Duration(budgetMS)*time.Millisecond, scanCount, 200000)
	ratio, estimated, min, max := estimateFromSample(dbsize, scanned, matched)

	return &prefixEstimate{
		Enabled:          true,
		BudgetMS:         budgetMS,
		ScanCount:        scanCount,
		ElapsedMS:        elapsedMS,
		ScannedKeys:      scanned,
		MatchedKeys:      matched,
		MatchedRatio:     ratio,
		EstimatedKeys:    estimated,
		EstimatedKeysMin: min,
		EstimatedKeysMax: max,
	}
}

func queryBool(c *app.RequestContext, key string) bool {
	v := strings.TrimSpace(string(c.Query(key)))
	if v == "" {
		return false
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "y"
}

func queryInt64Bounded(c *app.RequestContext, key string, def, min, max int64) int64 {
	v := strings.TrimSpace(string(c.Query(key)))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func samplePrefixKeys(ctx context.Context, rdb scanClient, prefix string, budget time.Duration, scanCount int64, maxScanned int64) (scanned int64, matched int64, elapsedMS int64) {
	start := time.Now()
	deadline := start.Add(budget)

	var cursor uint64
	for time.Now().Before(deadline) && scanned < maxScanned {
		keys, next, err := rdb.Scan(ctx, cursor, "", scanCount).Result()
		if err != nil {
			break
		}

		incScanned, incMatched, reached := countPrefixMatches(keys, prefix, maxScanned-scanned)
		scanned += incScanned
		matched += incMatched
		if reached {
			break
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	elapsedMS = time.Since(start).Milliseconds()
	return scanned, matched, elapsedMS
}

func countPrefixMatches(keys []string, prefix string, limit int64) (scanned int64, matched int64, reached bool) {
	if limit <= 0 {
		return 0, 0, true
	}
	for _, k := range keys {
		scanned++
		if prefix != "" && strings.HasPrefix(k, prefix) {
			matched++
		}
		if scanned >= limit {
			return scanned, matched, true
		}
	}
	return scanned, matched, false
}

func estimateFromSample(dbsize int64, scanned int64, matched int64) (ratio float64, estimated int64, min int64, max int64) {
	if scanned > 0 {
		ratio = float64(matched) / float64(scanned)
	}
	estimated = int64(float64(dbsize) * ratio)

	// Pragmatic uncertainty range (not a guarantee).
	factor := float64(1)
	if scanned < 20000 {
		factor = 2
	} else if scanned < 100000 {
		factor = 1.5
	}
	min = int64(float64(estimated) / factor)
	max = int64(float64(estimated) * factor)
	if dbsize == 0 {
		min, max = 0, 0
	}
	return ratio, estimated, min, max
}
