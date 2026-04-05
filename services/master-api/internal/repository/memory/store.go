package memory

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
	"github.com/gofxq/gaoming/pkg/state"
)

var ErrHostNotFound = errors.New("host not found")

const loadHistoryRetention = time.Hour
const heartbeatOfflineThreshold = 15 * time.Second

type MaintenanceWindow struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	ScopeType string    `json:"scope_type"`
	ScopeRef  string    `json:"scope_ref"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	CreatedBy string    `json:"created_by"`
	Reason    string    `json:"reason,omitempty"`
}

type Store struct {
	mu          sync.RWMutex
	hosts       map[string]state.HostSnapshot
	histories   map[string]map[state.MetricKey][]state.MetricPoint
	configs     map[string]contracts.AgentConfig
	maintenance []MaintenanceWindow
	alertAcks   map[string]string
	watchers    map[string]chan []state.HostSnapshot
}

func NewStore() *Store {
	return &Store{
		hosts:     make(map[string]state.HostSnapshot),
		histories: make(map[string]map[state.MetricKey][]state.MetricPoint),
		configs:   make(map[string]contracts.AgentConfig),
		alertAcks: make(map[string]string),
		watchers:  make(map[string]chan []state.HostSnapshot),
	}
}

func (s *Store) RegisterAgent(req contracts.RegisterAgentRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig) {
	s.mu.Lock()

	hostUID := req.Host.HostUID
	if hostUID == "" {
		hostUID = ids.New("host")
	}

	config := contracts.AgentConfig{
		ConfigVersion:        1,
		HeartbeatIntervalSec: 1,
		MetricIntervalSec:    1,
		StaticLabels:         req.Host.Labels,
	}

	snapshot := s.hosts[hostUID]
	snapshot.HostUID = hostUID
	snapshot.Hostname = req.Host.Hostname
	snapshot.PrimaryIP = req.Host.PrimaryIP
	snapshot.AgentState = state.Up
	snapshot.ReachabilityState = state.Unknown
	snapshot.ServiceState = state.Unknown
	snapshot.OverallState = state.Up
	snapshot.Labels = cloneLabels(req.Host.Labels)
	snapshot.LastAgentSeenAt = now
	snapshot.Version++

	s.hosts[hostUID] = snapshot
	if _, ok := s.histories[hostUID]; !ok {
		s.histories[hostUID] = make(map[state.MetricKey][]state.MetricPoint)
	}
	s.configs[hostUID] = config
	watchers, items := s.snapshotWatchersLocked()
	s.mu.Unlock()
	s.broadcast(watchers, items)
	return snapshot, config
}

func (s *Store) Heartbeat(req contracts.HeartbeatRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, error) {
	s.mu.Lock()

	snapshot, ok := s.hosts[req.HostUID]
	if !ok {
		s.mu.Unlock()
		return state.HostSnapshot{}, contracts.AgentConfig{}, ErrHostNotFound
	}

	snapshot.AgentState = state.Up
	snapshot.OverallState = state.Up
	snapshot.CPUUsagePct = req.Digest.CPUUsagePct
	snapshot.MemUsedPct = req.Digest.MemUsedPct
	snapshot.DiskUsedPct = req.Digest.DiskUsedPct
	snapshot.DiskReadBPS = req.Digest.DiskReadBPS
	snapshot.DiskWriteBPS = req.Digest.DiskWriteBPS
	snapshot.Load1 = req.Digest.Load1
	snapshot.NetRxBPS = req.Digest.NetRxBPS
	snapshot.NetTxBPS = req.Digest.NetTxBPS
	snapshot.LastAgentSeenAt = now
	snapshot.LastMetricAt = now
	snapshot.Version++

	s.hosts[req.HostUID] = snapshot
	s.recordMetricLocked(req.HostUID, state.MetricCPUUsagePct, now, snapshot.CPUUsagePct)
	s.recordMetricLocked(req.HostUID, state.MetricMemUsedPct, now, snapshot.MemUsedPct)
	s.recordMetricLocked(req.HostUID, state.MetricDiskUsedPct, now, snapshot.DiskUsedPct)
	s.recordMetricLocked(req.HostUID, state.MetricDiskReadBPS, now, float64(snapshot.DiskReadBPS))
	s.recordMetricLocked(req.HostUID, state.MetricDiskWriteBPS, now, float64(snapshot.DiskWriteBPS))
	s.recordMetricLocked(req.HostUID, state.MetricLoad1, now, snapshot.Load1)
	s.recordMetricLocked(req.HostUID, state.MetricNetRxBPS, now, float64(snapshot.NetRxBPS))
	s.recordMetricLocked(req.HostUID, state.MetricNetTxBPS, now, float64(snapshot.NetTxBPS))
	watchers, items := s.snapshotWatchersLocked()
	s.mu.Unlock()
	s.broadcast(watchers, items)
	return snapshot, s.configs[req.HostUID], nil
}

func (s *Store) ListHosts() []state.HostSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]state.HostSnapshot, 0, len(s.hosts))
	for _, host := range s.hosts {
		items = append(items, host)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].HostUID < items[j].HostUID
	})
	return items
}

func (s *Store) GetHost(hostUID string) (state.HostSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, ok := s.hosts[hostUID]
	return host, ok
}

func (s *Store) GetMetricHistory(hostUID string) map[state.MetricKey][]state.MetricPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := s.histories[hostUID]
	return cloneMetricHistory(history)
}

func (s *Store) GetAllMetricHistory() map[string]map[state.MetricKey][]state.MetricPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[state.MetricKey][]state.MetricPoint, len(s.histories))
	for hostUID, history := range s.histories {
		result[hostUID] = cloneMetricHistory(history)
	}
	return result
}

func (s *Store) CreateMaintenance(req contracts.CreateMaintenanceWindowRequest) MaintenanceWindow {
	s.mu.Lock()
	defer s.mu.Unlock()

	window := MaintenanceWindow{
		ID:        ids.New("maint"),
		Title:     req.Title,
		ScopeType: req.ScopeType,
		ScopeRef:  req.ScopeRef,
		StartAt:   req.StartAt,
		EndAt:     req.EndAt,
		CreatedBy: req.CreatedBy,
		Reason:    req.Reason,
	}

	s.maintenance = append(s.maintenance, window)
	return window
}

func (s *Store) AckAlert(alertID string, ackedBy string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertAcks[alertID] = ackedBy
}

func (s *Store) ReconcileOffline(now time.Time) int {
	s.mu.Lock()

	changed := 0
	for hostUID, snapshot := range s.hosts {
		if snapshot.LastAgentSeenAt.IsZero() {
			continue
		}
		if now.Sub(snapshot.LastAgentSeenAt) <= heartbeatOfflineThreshold {
			continue
		}
		if snapshot.AgentState == state.Offline && snapshot.OverallState == state.Offline {
			continue
		}

		snapshot.AgentState = state.Offline
		snapshot.ReachabilityState = state.Offline
		snapshot.OverallState = state.Offline
		snapshot.Version++
		s.hosts[hostUID] = snapshot
		changed++
	}

	if changed == 0 {
		s.mu.Unlock()
		return 0
	}

	watchers, items := s.snapshotWatchersLocked()
	s.mu.Unlock()
	s.broadcast(watchers, items)
	return changed
}

func (s *Store) Subscribe() (string, <-chan []state.HostSnapshot, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := ids.New("watch")
	ch := make(chan []state.HostSnapshot, 1)
	s.watchers[id] = ch
	ch <- s.listHostsLocked()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		_, ok := s.watchers[id]
		if !ok {
			return
		}

		delete(s.watchers, id)
	}

	return id, ch, cancel
}

func (s *Store) snapshotWatchersLocked() ([]chan []state.HostSnapshot, []state.HostSnapshot) {
	watchers := make([]chan []state.HostSnapshot, 0, len(s.watchers))
	for _, watcher := range s.watchers {
		watchers = append(watchers, watcher)
	}
	return watchers, s.listHostsLocked()
}

func (s *Store) listHostsLocked() []state.HostSnapshot {
	items := make([]state.HostSnapshot, 0, len(s.hosts))
	for _, host := range s.hosts {
		items = append(items, host)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].HostUID < items[j].HostUID
	})
	return items
}

func (s *Store) broadcast(watchers []chan []state.HostSnapshot, items []state.HostSnapshot) {
	for _, watcher := range watchers {
		select {
		case watcher <- items:
		default:
			select {
			case <-watcher:
			default:
			}
			select {
			case watcher <- items:
			default:
			}
		}
	}
}

func cloneMetricPoints(points []state.MetricPoint) []state.MetricPoint {
	if len(points) == 0 {
		return nil
	}

	cloned := make([]state.MetricPoint, len(points))
	copy(cloned, points)
	return cloned
}

func cloneMetricHistory(history map[state.MetricKey][]state.MetricPoint) map[state.MetricKey][]state.MetricPoint {
	if len(history) == 0 {
		return nil
	}

	cloned := make(map[state.MetricKey][]state.MetricPoint, len(history))
	for key, points := range history {
		cloned[key] = cloneMetricPoints(points)
	}
	return cloned
}

func (s *Store) recordMetricLocked(hostUID string, key state.MetricKey, now time.Time, value float64) {
	if _, ok := s.histories[hostUID]; !ok {
		s.histories[hostUID] = make(map[state.MetricKey][]state.MetricPoint)
	}
	s.histories[hostUID][key] = pruneMetricHistory(append(s.histories[hostUID][key], state.MetricPoint{
		TS:    now,
		Value: value,
	}), now)
}

func pruneMetricHistory(points []state.MetricPoint, now time.Time) []state.MetricPoint {
	cutoff := now.Add(-loadHistoryRetention)
	idx := 0
	for idx < len(points) && points[idx].TS.Before(cutoff) {
		idx++
	}
	if idx == 0 {
		return points
	}
	return append([]state.MetricPoint(nil), points[idx:]...)
}

func cloneLabels(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
