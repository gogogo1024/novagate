package acl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	c      *redis.Client
	prefix string
}

var _ Store = (*RedisStore)(nil)

var (
	redisScriptSRemDelIfEmpty = redis.NewScript(
		"redis.call('SREM', KEYS[1], ARGV[1]); " +
			"if redis.call('SCARD', KEYS[1]) == 0 then redis.call('DEL', KEYS[1]); end; " +
			"return 1",
	)
	redisScriptZRemDelIfEmpty = redis.NewScript(
		"redis.call('ZREM', KEYS[1], ARGV[1]); " +
			"if redis.call('ZCARD', KEYS[1]) == 0 then redis.call('DEL', KEYS[1]); end; " +
			"return 1",
	)
)

func NewRedisStore(c *redis.Client, keyPrefix string) *RedisStore {
	if keyPrefix == "" {
		keyPrefix = "acl:"
	}
	return &RedisStore{c: c, prefix: keyPrefix}
}

func (s *RedisStore) Client() *redis.Client {
	return s.c
}

func (s *RedisStore) Prefix() string {
	return s.prefix
}

func (s *RedisStore) SetVisibility(tenantID, docID string, v Visibility) error {
	if v != VisibilityPublic && v != VisibilityRestricted {
		return ErrInvalidVisibility
	}
	if tenantID == "" || docID == "" {
		return errors.New("tenant_id/doc_id is required")
	}
	ctx := context.Background()
	key := s.keyVisibility(tenantID, docID)
	// Missing key means public by default.
	if v == VisibilityPublic {
		return s.c.Del(ctx, key).Err()
	}
	return s.c.Set(ctx, key, string(v), 0).Err()
}

func (s *RedisStore) Grant(tenantID, docID, userID string, validFrom time.Time, validTo *time.Time) error {
	if tenantID == "" || docID == "" || userID == "" {
		return errors.New("tenant_id/doc_id/user_id is required")
	}
	if !validFrom.IsZero() && validTo != nil && !validTo.IsZero() {
		if validTo.Before(validFrom) {
			return errors.New("valid_to must be >= valid_from")
		}
	}

	ctx := context.Background()
	pipe := s.c.Pipeline()

	docPermKey := s.keyPermanent(tenantID, docID)
	docExpKey := s.keyExpiring(tenantID, docID)
	userPermKey := s.keyUserPermanent(tenantID, userID)
	userExpKey := s.keyUserExpiring(tenantID, userID)

	if validTo == nil {
		pipe.SAdd(ctx, docPermKey, userID)
		pipe.SAdd(ctx, userPermKey, docID)
		pipe.ZRem(ctx, docExpKey, userID)
		pipe.ZRem(ctx, userExpKey, docID)
	} else {
		// Opportunistic cleanup to avoid unbounded growth of expired members.
		nowUnix := time.Now().Unix()
		max := fmt.Sprintf("%d", nowUnix)
		pipe.ZRemRangeByScore(ctx, docExpKey, "-inf", max)
		pipe.ZRemRangeByScore(ctx, userExpKey, "-inf", max)

		score := float64(validTo.Unix())
		pipe.ZAdd(ctx, docExpKey, redis.Z{Score: score, Member: userID})
		pipe.ZAdd(ctx, userExpKey, redis.Z{Score: score, Member: docID})
		pipe.SRem(ctx, docPermKey, userID)
		pipe.SRem(ctx, userPermKey, docID)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	// Best-effort cleanup to reduce empty key count.
	cleanup := s.c.Pipeline()
	if validTo == nil {
		_ = redisScriptZRemDelIfEmpty.Run(ctx, cleanup, []string{docExpKey}, userID)
		_ = redisScriptZRemDelIfEmpty.Run(ctx, cleanup, []string{userExpKey}, docID)
	} else {
		_ = redisScriptSRemDelIfEmpty.Run(ctx, cleanup, []string{docPermKey}, userID)
		_ = redisScriptSRemDelIfEmpty.Run(ctx, cleanup, []string{userPermKey}, docID)
	}
	_, _ = cleanup.Exec(ctx)

	return nil
}

func (s *RedisStore) Revoke(tenantID, docID, userID string) error {
	if tenantID == "" || docID == "" || userID == "" {
		return errors.New("tenant_id/doc_id/user_id is required")
	}
	ctx := context.Background()
	pipe := s.c.Pipeline()

	_ = redisScriptSRemDelIfEmpty.Run(ctx, pipe, []string{s.keyPermanent(tenantID, docID)}, userID)
	_ = redisScriptZRemDelIfEmpty.Run(ctx, pipe, []string{s.keyExpiring(tenantID, docID)}, userID)
	_ = redisScriptSRemDelIfEmpty.Run(ctx, pipe, []string{s.keyUserPermanent(tenantID, userID)}, docID)
	_ = redisScriptZRemDelIfEmpty.Run(ctx, pipe, []string{s.keyUserExpiring(tenantID, userID)}, docID)

	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) CheckBatch(tenantID, userID string, docIDs []string, now time.Time) []string {
	if tenantID == "" || userID == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	ctx := context.Background()
	userPermKey := s.keyUserPermanent(tenantID, userID)
	userExpKey := s.keyUserExpiring(tenantID, userID)
	ops := make([]redisDocOps, 0, len(docIDs))
	filtered := make([]string, 0, len(docIDs))

	pipe := s.c.Pipeline()

	for _, docID := range docIDs {
		if docID == "" {
			continue
		}
		filtered = append(filtered, docID)
		ops = append(ops, s.enqueueDocOps(pipe, tenantID, docID, userPermKey, userExpKey))
	}

	_, _ = pipe.Exec(ctx)

	allowed := make([]string, 0, len(filtered))
	nowUnix := float64(now.Unix())
	for i, docID := range filtered {
		op := ops[i]
		if visibilityFromCmd(op.visCmd) == VisibilityPublic {
			allowed = append(allowed, docID)
			continue
		}
		if isAllowedRestricted(op.permCmd, op.expCmd, nowUnix) {
			allowed = append(allowed, docID)
			continue
		}
	}

	return allowed
}

type redisDocOps struct {
	visCmd  *redis.StringCmd
	permCmd *redis.BoolCmd
	expCmd  *redis.FloatCmd
}

func (s *RedisStore) enqueueDocOps(pipe redis.Pipeliner, tenantID, docID, userPermKey, userExpKey string) redisDocOps {
	ctx := context.Background()
	return redisDocOps{
		visCmd:  pipe.Get(ctx, s.keyVisibility(tenantID, docID)),
		permCmd: pipe.SIsMember(ctx, userPermKey, docID),
		expCmd:  pipe.ZScore(ctx, userExpKey, docID),
	}
}

func (s *RedisStore) ListGrants(tenantID, userID string, now time.Time) []string {
	if tenantID == "" || userID == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	ctx := context.Background()
	userPermKey := s.keyUserPermanent(tenantID, userID)
	userExpKey := s.keyUserExpiring(tenantID, userID)

	permDocs, err := s.c.SMembers(ctx, userPermKey).Result()
	if err != nil {
		return nil
	}

	min := fmt.Sprintf("(%d", now.Unix())
	expDocs, err := s.c.ZRangeByScore(ctx, userExpKey, &redis.ZRangeBy{Min: min, Max: "+inf"}).Result()
	if err != nil {
		return permDocs
	}

	seen := make(map[string]struct{}, len(permDocs)+len(expDocs))
	for _, d := range permDocs {
		if d != "" {
			seen[d] = struct{}{}
		}
	}
	for _, d := range expDocs {
		if d != "" {
			seen[d] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	return out
}

func (s *RedisStore) RevokeAllUser(tenantID, userID string) error {
	if tenantID == "" || userID == "" {
		return errors.New("tenant_id/user_id is required")
	}
	ctx := context.Background()
	userPermKey := s.keyUserPermanent(tenantID, userID)
	userExpKey := s.keyUserExpiring(tenantID, userID)

	permDocs, err := s.c.SMembers(ctx, userPermKey).Result()
	if err != nil {
		return err
	}
	expDocs, err := s.c.ZRange(ctx, userExpKey, 0, -1).Result()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(permDocs)+len(expDocs))
	for _, d := range permDocs {
		if d != "" {
			seen[d] = struct{}{}
		}
	}
	for _, d := range expDocs {
		if d != "" {
			seen[d] = struct{}{}
		}
	}

	pipe := s.c.Pipeline()
	for docID := range seen {
		_ = redisScriptSRemDelIfEmpty.Run(ctx, pipe, []string{s.keyPermanent(tenantID, docID)}, userID)
		_ = redisScriptZRemDelIfEmpty.Run(ctx, pipe, []string{s.keyExpiring(tenantID, docID)}, userID)
	}
	pipe.Del(ctx, userPermKey, userExpKey)
	_, err = pipe.Exec(ctx)
	return err
}

func visibilityFromCmd(cmd *redis.StringCmd) Visibility {
	if cmd == nil {
		return VisibilityPublic
	}
	vv, err := cmd.Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return VisibilityPublic
		}
		return VisibilityPublic
	}
	if Visibility(vv) == VisibilityRestricted {
		return VisibilityRestricted
	}
	return VisibilityPublic
}

func isAllowedRestricted(permCmd *redis.BoolCmd, expCmd *redis.FloatCmd, nowUnix float64) bool {
	if permCmd != nil {
		ok, err := permCmd.Result()
		if err == nil && ok {
			return true
		}
	}
	if expCmd != nil {
		score, err := expCmd.Result()
		if err == nil {
			return nowUnix < score
		}
	}
	return false
}

func (s *RedisStore) keyVisibility(tenantID, docID string) string {
	return fmt.Sprintf("%st:%s:doc:%s:vis", s.prefix, tenantID, docID)
}

func (s *RedisStore) keyPermanent(tenantID, docID string) string {
	return fmt.Sprintf("%st:%s:doc:%s:perm", s.prefix, tenantID, docID)
}

func (s *RedisStore) keyExpiring(tenantID, docID string) string {
	return fmt.Sprintf("%st:%s:doc:%s:exp", s.prefix, tenantID, docID)
}

func (s *RedisStore) keyUserPermanent(tenantID, userID string) string {
	return fmt.Sprintf("%st:%s:u:%s:perm", s.prefix, tenantID, userID)
}

func (s *RedisStore) keyUserExpiring(tenantID, userID string) string {
	return fmt.Sprintf("%st:%s:u:%s:exp", s.prefix, tenantID, userID)
}
