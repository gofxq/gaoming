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
	configs     map[string]contracts.AgentConfig
	maintenance []MaintenanceWindow
	alertAcks   map[string]string
}

func NewStore() *Store {
	return &Store{
		hosts:     make(map[string]state.HostSnapshot),
		configs:   make(map[string]contracts.AgentConfig),
		alertAcks: make(map[string]string),
	}
}

func (s *Store) RegisterAgent(req contracts.RegisterAgentRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hostUID := req.Host.HostUID
	if hostUID == "" {
		hostUID = ids.New("host")
	}

	config := contracts.AgentConfig{
		ConfigVersion:        1,
		HeartbeatIntervalSec: 5,
		MetricIntervalSec:    5,
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
	s.configs[hostUID] = config
	return snapshot, config
}

func (s *Store) Heartbeat(req contracts.HeartbeatRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.hosts[req.HostUID]
	if !ok {
		return state.HostSnapshot{}, contracts.AgentConfig{}, ErrHostNotFound
	}

	snapshot.AgentState = state.Up
	snapshot.OverallState = state.Up
	snapshot.CPUUsagePct = req.Digest.CPUUsagePct
	snapshot.MemUsedPct = req.Digest.MemUsedPct
	snapshot.DiskUsedPct = req.Digest.DiskUsedPct
	snapshot.Load1 = req.Digest.Load1
	snapshot.NetRxBPS = req.Digest.NetRxBPS
	snapshot.NetTxBPS = req.Digest.NetTxBPS
	snapshot.LastAgentSeenAt = now
	snapshot.LastMetricAt = now
	snapshot.Version++

	s.hosts[req.HostUID] = snapshot
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
