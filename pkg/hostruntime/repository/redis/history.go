package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/state"
	goredis "github.com/redis/go-redis/v9"
)

type MetricWindowStore struct {
	client    *goredis.Client
	keyPrefix string
	maxPoints int64
	windowTTL time.Duration
}

func NewMetricWindowStore(client *goredis.Client, keyPrefix string, maxPoints int64, windowTTL time.Duration) *MetricWindowStore {
	if keyPrefix == "" {
		keyPrefix = "gaoming:metrics"
	}
	if maxPoints <= 0 {
		maxPoints = 3600
	}
	if windowTTL <= 0 {
		windowTTL = 2 * time.Hour
	}
	return &MetricWindowStore{
		client:    client,
		keyPrefix: keyPrefix,
		maxPoints: maxPoints,
		windowTTL: windowTTL,
	}
}

func (s *MetricWindowStore) AppendHeartbeatMetrics(ctx context.Context, hostUID string, now time.Time, digest contracts.AgentDigest) error {
	pipe := s.client.Pipeline()
	for key, value := range repository.DigestMetricValues(digest) {
		body, err := json.Marshal(state.MetricPoint{TS: now, Value: value})
		if err != nil {
			return err
		}
		redisKey := s.metricKey(hostUID, key)
		pipe.LPush(ctx, redisKey, body)
		pipe.LTrim(ctx, redisKey, 0, s.maxPoints-1)
		pipe.Expire(ctx, redisKey, s.windowTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *MetricWindowStore) GetHostMetricHistory(ctx context.Context, hostUID string) (map[state.MetricKey][]state.MetricPoint, error) {
	keys := repository.MetricKeys()
	pipe := s.client.Pipeline()
	cmds := make(map[state.MetricKey]*goredis.StringSliceCmd, len(keys))
	for _, key := range keys {
		cmds[key] = pipe.LRange(ctx, s.metricKey(hostUID, key), 0, -1)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != goredis.Nil {
		return nil, err
	}

	result := make(map[state.MetricKey][]state.MetricPoint, len(keys))
	for _, key := range keys {
		points, err := decodeMetricPoints(cmds[key].Val())
		if err != nil {
			return nil, err
		}
		if len(points) > 0 {
			result[key] = points
		}
	}
	return result, nil
}

func (s *MetricWindowStore) GetAllHostMetricHistory(ctx context.Context, hostUIDs []string) (map[string]map[state.MetricKey][]state.MetricPoint, error) {
	result := make(map[string]map[state.MetricKey][]state.MetricPoint, len(hostUIDs))
	for _, hostUID := range hostUIDs {
		history, err := s.GetHostMetricHistory(ctx, hostUID)
		if err != nil {
			return nil, err
		}
		if len(history) > 0 {
			result[hostUID] = history
		}
	}
	return result, nil
}

func (s *MetricWindowStore) metricKey(hostUID string, key state.MetricKey) string {
	return fmt.Sprintf("%s:%s:%s", s.keyPrefix, hostUID, key)
}

func decodeMetricPoints(raw []string) ([]state.MetricPoint, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	points := make([]state.MetricPoint, 0, len(raw))
	for i := len(raw) - 1; i >= 0; i-- {
		var point state.MetricPoint
		if err := json.Unmarshal([]byte(raw[i]), &point); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}
