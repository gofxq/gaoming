// Package access defines identities derived from verified credentials. Request
// payloads never construct trusted principals.
package access

import (
	"errors"
	"strconv"
)

type Kind string

const (
	User   Kind = "user"
	Share  Kind = "share"
	Demo   Kind = "demo"
	Agent  Kind = "agent"
	Worker Kind = "worker"
)

var ErrForbidden = errors.New("forbidden")
var ErrUnauthorized = errors.New("unauthorized")
var ErrNotFound = errors.New("resource not found")
var ErrQuota = errors.New("access quota exceeded")

type Principal struct {
	Kind         Kind
	TenantID     int64
	TenantCode   string
	UserID       int64
	Role         string
	HostUID      string
	WorkerID     string
	ShareID      string
	AllowedHosts []string
	Fields       []string
}

func (p Principal) IsUser() bool {
	return p.Kind == User && p.UserID > 0 && p.TenantCode != "" && (p.Role == "admin" || p.Role == "member")
}
func (p Principal) IsAdmin() bool { return p.IsUser() && p.Role == "admin" }
func (p Principal) Actor() string {
	if !p.IsUser() {
		return ""
	}
	return "user:" + strconv.FormatInt(p.UserID, 10)
}
func (p Principal) AllowsHost(tenant, uid string) bool {
	if p.TenantCode == "" || tenant != p.TenantCode || uid == "" {
		return false
	}
	if p.IsUser() {
		return true
	}
	if p.Kind != Share && p.Kind != Demo {
		return false
	}
	for _, allowed := range p.AllowedHosts {
		if uid == allowed {
			return true
		}
	}
	return false
}
func (p Principal) AllowsField(field string) bool {
	if p.IsUser() {
		return true
	}
	for _, allowed := range p.Fields {
		if field == allowed {
			return true
		}
	}
	return false
}
