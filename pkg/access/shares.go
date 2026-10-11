package access

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type CreateShareRequest struct {
	Title      string   `json:"title"`
	HostUIDs   []string `json:"host_uids"`
	Fields     []string `json:"fields"`
	TTLSeconds int      `json:"ttl_seconds"`
}

type ShareRecord struct {
	ID         string     `json:"id"`
	TenantCode string     `json:"tenant_code"`
	Title      string     `json:"title"`
	HostUIDs   []string   `json:"host_uids"`
	Fields     []string   `json:"fields"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type ShareSession struct {
	Token     string
	ExpiresAt time.Time
	Principal Principal
}

type ShareStore interface {
	CreateShare(context.Context, Principal, CreateShareRequest, string, time.Time) (ShareRecord, error)
	ListShares(context.Context, Principal) ([]ShareRecord, error)
	RevokeShare(context.Context, Principal, string, time.Time) error
	Authenticate(context.Context, string, Kind, time.Time) (Principal, error)
	ExchangeShare(context.Context, string, time.Time) (ShareSession, error)
	AuthenticateShareSession(context.Context, string, time.Time) (Principal, error)
	ValidateDemo(context.Context, Principal) error
}

func ValidateShareRequest(req CreateShareRequest) error {
	if strings.TrimSpace(req.Title) == "" || len(req.Title) > 128 || len(req.HostUIDs) == 0 || len(req.HostUIDs) > 100 || req.TTLSeconds < 60 || req.TTLSeconds > 7*24*3600 || len(req.Fields) == 0 || len(req.Fields) > 3 {
		return fmt.Errorf("invalid share title, hosts, fields or lifetime")
	}
	hosts := map[string]bool{}
	for _, uid := range req.HostUIDs {
		if uid == "" || len(uid) > 64 || hosts[uid] || strings.TrimSpace(uid) != uid {
			return fmt.Errorf("invalid or duplicate share host")
		}
		hosts[uid] = true
	}
	fields := map[string]bool{}
	for _, field := range req.Fields {
		if fields[field] || (field != "hostname" && field != "status" && field != "metrics") {
			return fmt.Errorf("invalid or duplicate share field")
		}
		fields[field] = true
	}
	return nil
}
