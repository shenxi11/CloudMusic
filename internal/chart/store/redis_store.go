package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	chartmodel "music-platform/internal/chart/model"

	"github.com/go-redis/redis/v8"
)

var ErrUnavailable = errors.New("redis leaderboard store is unavailable")

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Available() bool {
	return s != nil && s.client != nil
}

func (s *RedisStore) IncrementPlay(ctx context.Context, totalKey, dayKey, musicPath string, dayTTL time.Duration) error {
	if !s.Available() {
		return ErrUnavailable
	}

	pipe := s.client.TxPipeline()
	pipe.ZIncrBy(ctx, totalKey, 1, musicPath)
	pipe.ZIncrBy(ctx, dayKey, 1, musicPath)
	if dayTTL > 0 {
		pipe.Expire(ctx, dayKey, dayTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) UpsertMeta(ctx context.Context, key string, meta *chartmodel.HotTrackMeta, ttl time.Duration) error {
	if !s.Available() {
		return ErrUnavailable
	}
	if meta == nil {
		return nil
	}
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, payload, ttl).Err()
}

func (s *RedisStore) GetMeta(ctx context.Context, key string) (*chartmodel.HotTrackMeta, bool, error) {
	if !s.Available() {
		return nil, false, ErrUnavailable
	}
	raw, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var meta chartmodel.HotTrackMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, false, err
	}
	return &meta, true, nil
}

func (s *RedisStore) TopN(ctx context.Context, key string, limit int64) ([]chartmodel.ScoredMusicPath, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.client.ZRevRangeWithScores(ctx, key, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]chartmodel.ScoredMusicPath, 0, len(rows))
	for _, row := range rows {
		member, ok := row.Member.(string)
		if !ok {
			continue
		}
		out = append(out, chartmodel.ScoredMusicPath{
			MusicPath: member,
			Score:     row.Score,
		})
	}
	return out, nil
}

func (s *RedisStore) UnionInto(ctx context.Context, dest string, keys []string, ttl time.Duration) error {
	if !s.Available() {
		return ErrUnavailable
	}
	if len(keys) == 0 {
		return s.Delete(ctx, dest)
	}
	pipe := s.client.TxPipeline()
	pipe.ZUnionStore(ctx, dest, &redis.ZStore{Keys: keys})
	if ttl > 0 {
		pipe.Expire(ctx, dest, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) ReplaceLeaderboard(ctx context.Context, key string, scores map[string]float64, ttl time.Duration) error {
	if !s.Available() {
		return ErrUnavailable
	}

	pipe := s.client.TxPipeline()
	pipe.Del(ctx, key)
	if len(scores) > 0 {
		values := make([]*redis.Z, 0, len(scores))
		for musicPath, score := range scores {
			values = append(values, &redis.Z{Score: score, Member: musicPath})
		}
		pipe.ZAdd(ctx, key, values...)
		if ttl > 0 {
			pipe.Expire(ctx, key, ttl)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) Delete(ctx context.Context, keys ...string) error {
	if !s.Available() {
		return ErrUnavailable
	}
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}
