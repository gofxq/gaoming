package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofxq/gaoming/pkg/access"
	"github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/state"
)

type CreatedShare struct {
	access.ShareRecord
	Token string `json:"token"`
}

// ConfigureAccess is called once during application assembly before serving.
func (s *Service) ConfigureAccess(store access.ShareStore, demo access.Principal) {
	s.shareStore = store
	s.demo = demo
}

func (s *Service) CreateShare(ctx context.Context, p access.Principal, req access.CreateShareRequest) (CreatedShare, error) {
	if !p.IsAdmin() {
		return CreatedShare{}, ErrForbidden
	}
	if s.shareStore == nil {
		return CreatedShare{}, ErrAuthNotConfigured
	}
	req.Title = strings.TrimSpace(req.Title)
	if err := access.ValidateShareRequest(req); err != nil {
		return CreatedShare{}, err
	}
	token, hash, err := access.NewToken()
	if err != nil {
		return CreatedShare{}, fmt.Errorf("share creation unavailable")
	}
	record, err := s.shareStore.CreateShare(ctx, p, req, hash, s.clock.Now().UTC())
	if err != nil {
		return CreatedShare{}, err
	}
	return CreatedShare{ShareRecord: record, Token: token}, nil
}

func (s *Service) ListShares(ctx context.Context, p access.Principal) ([]access.ShareRecord, error) {
	if !p.IsAdmin() {
		return nil, ErrForbidden
	}
	if s.shareStore == nil {
		return nil, ErrAuthNotConfigured
	}
	return s.shareStore.ListShares(ctx, p)
}

func (s *Service) RevokeShare(ctx context.Context, p access.Principal, id string) error {
	if !p.IsAdmin() {
		return ErrForbidden
	}
	if s.shareStore == nil {
		return ErrAuthNotConfigured
	}
	return s.shareStore.RevokeShare(ctx, p, id, s.clock.Now().UTC())
}

func (s *Service) ExchangeShare(ctx context.Context, token string) (access.ShareSession, error) {
	if s.shareStore == nil {
		return access.ShareSession{}, ErrUnauthorized
	}
	return s.shareStore.ExchangeShare(ctx, token, s.clock.Now().UTC())
}

func (s *Service) AuthenticateShareSession(ctx context.Context, token string) (access.Principal, error) {
	if s.shareStore == nil {
		return access.Principal{}, ErrUnauthorized
	}
	return s.shareStore.AuthenticateShareSession(ctx, token, s.clock.Now().UTC())
}

func (s *Service) DemoPrincipal(ctx context.Context) (access.Principal, error) {
	if s.demo.Kind != access.Demo || s.demo.TenantCode == "" || len(s.demo.AllowedHosts) == 0 || len(s.demo.Fields) == 0 {
		return access.Principal{}, ErrForbidden
	}
	if s.shareStore == nil {
		return access.Principal{}, ErrForbidden
	}
	if err := s.shareStore.ValidateDemo(ctx, s.demo); err != nil {
		return access.Principal{}, err
	}
	return s.demo, nil
}

func validReader(p access.Principal) bool {
	return p.TenantCode != "" && (p.IsUser() || ((p.Kind == access.Share || p.Kind == access.Demo) && len(p.AllowedHosts) > 0 && len(p.Fields) > 0))
}

func (s *Service) ReadHosts(ctx context.Context, p access.Principal) ([]state.HostSnapshot, error) {
	if !validReader(p) {
		return nil, ErrUnauthorized
	}
	items, err := s.hostStore.ListHosts(ctx, p.TenantCode)
	if err != nil {
		return nil, err
	}
	result := make([]state.HostSnapshot, 0, len(items))
	for _, item := range items {
		if p.AllowsHost(item.TenantCode, item.HostUID) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Service) ReadHost(ctx context.Context, p access.Principal, uid string) (state.HostSnapshot, bool, error) {
	if !validReader(p) {
		return state.HostSnapshot{}, false, ErrUnauthorized
	}
	if !p.AllowsHost(p.TenantCode, uid) {
		return state.HostSnapshot{}, false, nil
	}
	host, ok, err := s.hostStore.GetHost(ctx, uid, p.TenantCode)
	if err != nil || !ok {
		return host, ok, err
	}
	if !p.AllowsHost(host.TenantCode, host.HostUID) {
		return state.HostSnapshot{}, false, nil
	}
	return host, true, nil
}

// ProjectHost uses an allowlist; zeroing a private struct is insufficient since
// its JSON fields would still disclose inventory or simulate measured zeroes.
func ProjectHost(p access.Principal, host state.HostSnapshot) any {
	if p.IsUser() {
		return host
	}
	view := map[string]any{"host_uid": host.HostUID}
	if p.AllowsField("hostname") {
		view["hostname"] = host.Hostname
	}
	if p.AllowsField("status") {
		view["agent_state"] = host.AgentState
		view["reachability_state"] = host.ReachabilityState
		view["service_state"] = host.ServiceState
		view["overall_state"] = host.OverallState
		view["last_agent_seen_at"] = host.LastAgentSeenAt
		view["last_probe_at"] = host.LastProbeAt
		view["open_alert_count"] = host.OpenAlertCount
		view["version"] = host.Version
	}
	if p.AllowsField("metrics") && !host.LastMetricAt.IsZero() {
		view["last_metric_at"] = host.LastMetricAt
		for key, value := range repository.DigestMetricValues(digestFromSnapshot(host)) {
			view[string(key)] = value
		}
	}
	return view
}

func ProjectHosts(p access.Principal, hosts []state.HostSnapshot) []any {
	result := make([]any, 0, len(hosts))
	for _, host := range hosts {
		result = append(result, ProjectHost(p, host))
	}
	return result
}

// Used by streams to fail closed on session expiration, role changes and link
// revocation. The transport owns its recheck timer and request deadlines.
func (s *Service) ReadAccessRecheckInterval() time.Duration { return time.Second }
