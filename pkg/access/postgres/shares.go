package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gofxq/gaoming/pkg/access"
	"github.com/gofxq/gaoming/pkg/ids"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})}
}

type shareRow struct {
	ID, TenantCode, Title string
	TenantID, CreatedBy   int64
	Fields                []byte
	ExpiresAt, CreatedAt  time.Time
	RevokedAt             *time.Time
}

func tenantForAdmin(tx *gorm.DB, p access.Principal) (int64, error) {
	if !p.IsAdmin() {
		return 0, access.ErrForbidden
	}
	var id int64
	err := tx.Raw(`SELECT users.tenant_id FROM users JOIN tenants ON tenants.id=users.tenant_id
		WHERE users.id=? AND tenants.tenant_code=? AND users.status='active' AND users.role='admin' AND tenants.status=1
		FOR SHARE OF users FOR UPDATE OF tenants`, p.UserID, p.TenantCode).Scan(&id).Error
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, access.ErrForbidden
	}
	return id, nil
}

func audit(tx *gorm.DB, p access.Principal, tenant int64, action, id string, now time.Time) error {
	return tx.Exec(`INSERT INTO audit_logs(tenant_id,actor,action,resource_type,resource_id,success,created_at) VALUES (?,?,?,'share',?,true,?)`, tenant, p.Actor(), action, id, now).Error
}

func (s *Store) CreateShare(ctx context.Context, p access.Principal, req access.CreateShareRequest, hash string, now time.Time) (access.ShareRecord, error) {
	if !p.IsAdmin() {
		return access.ShareRecord{}, access.ErrForbidden
	}
	if err := access.ValidateShareRequest(req); err != nil {
		return access.ShareRecord{}, err
	}
	if len(hash) != 64 {
		return access.ShareRecord{}, access.ErrUnauthorized
	}
	var record access.ShareRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := tenantForAdmin(tx, p)
		if err != nil {
			return err
		}
		// tenantForAdmin holds the exclusive tenant quota lock from the outset.
		var count int64
		if err := tx.Raw(`SELECT count(*) FROM access_shares WHERE tenant_id=?`, tenant).Scan(&count).Error; err != nil {
			return err
		}
		if count >= 100 {
			return access.ErrQuota
		}
		if err := tx.Table("hosts").Where("tenant_id=? AND host_uid IN ?", tenant, req.HostUIDs).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(req.HostUIDs)) {
			return access.ErrNotFound
		}
		fields, _ := json.Marshal(req.Fields)
		id := ids.New("share")
		expires := now.Add(time.Duration(req.TTLSeconds) * time.Second)
		if err := tx.Exec(`INSERT INTO access_shares(id,tenant_id,title,token_hash,fields,expires_at,created_by,created_at) VALUES (?,?,?,?,?::jsonb,?,?,?)`, id, tenant, req.Title, hash, string(fields), expires, p.UserID, now).Error; err != nil {
			return err
		}
		for _, uid := range req.HostUIDs {
			if err := tx.Exec(`INSERT INTO access_share_hosts(share_id,tenant_id,host_uid) VALUES (?,?,?)`, id, tenant, uid).Error; err != nil {
				return err
			}
		}
		if err := audit(tx, p, tenant, "share.create", id, now); err != nil {
			return err
		}
		record = access.ShareRecord{ID: id, TenantCode: p.TenantCode, Title: req.Title, HostUIDs: append([]string(nil), req.HostUIDs...), Fields: append([]string(nil), req.Fields...), ExpiresAt: expires, CreatedAt: now}
		return nil
	})
	return record, err
}

func (s *Store) ListShares(ctx context.Context, p access.Principal) ([]access.ShareRecord, error) {
	if !p.IsAdmin() {
		return nil, access.ErrForbidden
	}
	var result []access.ShareRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := tenantForAdmin(tx, p)
		if err != nil {
			return err
		}
		var rows []shareRow
		if err := tx.Raw(`SELECT access_shares.*,tenants.tenant_code FROM access_shares JOIN tenants ON tenants.id=tenant_id WHERE tenant_id=? ORDER BY created_at DESC LIMIT 100`, tenant).Scan(&rows).Error; err != nil {
			return err
		}
		result = make([]access.ShareRecord, 0, len(rows))
		for _, row := range rows {
			var fields, uids []string
			if err := json.Unmarshal(row.Fields, &fields); err != nil {
				return err
			}
			if err := tx.Table("access_share_hosts").Where("share_id=? AND tenant_id=?", row.ID, tenant).Order("host_uid").Pluck("host_uid", &uids).Error; err != nil {
				return err
			}
			result = append(result, access.ShareRecord{ID: row.ID, TenantCode: p.TenantCode, Title: row.Title, HostUIDs: uids, Fields: fields, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt, RevokedAt: row.RevokedAt})
		}
		return nil
	})
	return result, err
}

func (s *Store) RevokeShare(ctx context.Context, p access.Principal, id string, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, err := tenantForAdmin(tx, p)
		if err != nil {
			return err
		}
		var exists bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM access_shares WHERE id=? AND tenant_id=?)`, id, tenant).Scan(&exists).Error; err != nil {
			return err
		}
		if !exists {
			return access.ErrNotFound
		}
		if err := tx.Exec(`UPDATE access_shares SET revoked_at=COALESCE(revoked_at,?) WHERE id=? AND tenant_id=?`, now, id, tenant).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM access_share_sessions WHERE share_id=?`, id).Error; err != nil {
			return err
		}
		return audit(tx, p, tenant, "share.revoke", id, now)
	})
}

func validShare(db *gorm.DB, where string, args ...any) (shareRow, error) {
	var row shareRow
	err := db.Raw(`SELECT access_shares.*,tenants.tenant_code FROM access_shares JOIN tenants ON tenants.id=access_shares.tenant_id
		JOIN users ON users.id=access_shares.created_by AND users.tenant_id=access_shares.tenant_id
		WHERE tenants.status=1 AND users.status='active' AND users.role='admin' AND access_shares.revoked_at IS NULL AND `+where, args...).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, access.ErrUnauthorized
	}
	return row, err
}

func sharePrincipal(db *gorm.DB, row shareRow) (access.Principal, error) {
	p := access.Principal{Kind: access.Share, TenantID: row.TenantID, TenantCode: row.TenantCode, ShareID: row.ID}
	if err := json.Unmarshal(row.Fields, &p.Fields); err != nil {
		return p, access.ErrUnauthorized
	}
	if err := db.Table("access_share_hosts").Where("share_id=? AND tenant_id=?", row.ID, row.TenantID).Order("host_uid").Pluck("host_uid", &p.AllowedHosts).Error; err != nil {
		return p, err
	}
	return p, nil
}

func (s *Store) Authenticate(ctx context.Context, raw string, kind access.Kind, now time.Time) (access.Principal, error) {
	if kind != access.Share || access.TokenHash(raw) == "" {
		return access.Principal{}, access.ErrUnauthorized
	}
	db := s.db.WithContext(ctx)
	row, err := validShare(db, "access_shares.token_hash=? AND access_shares.expires_at>?", access.TokenHash(raw), now)
	if err != nil {
		return access.Principal{}, err
	}
	return sharePrincipal(db, row)
}

func (s *Store) ExchangeShare(ctx context.Context, raw string, now time.Time) (access.ShareSession, error) {
	if access.TokenHash(raw) == "" {
		return access.ShareSession{}, access.ErrUnauthorized
	}
	var result access.ShareSession
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the share before verifying validity/creating a session; revocation
		// acquires the same row lock and cannot be overtaken by exchange.
		var id string
		if err := tx.Raw(`SELECT id FROM access_shares WHERE token_hash=? FOR UPDATE`, access.TokenHash(raw)).Scan(&id).Error; err != nil {
			return err
		}
		if id == "" {
			return access.ErrUnauthorized
		}
		row, err := validShare(tx, "access_shares.id=? AND access_shares.expires_at>?", id, now)
		if err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM access_share_sessions WHERE share_id=? AND expires_at<=?`, id, now).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Table("access_share_sessions").Where("share_id=?", id).Count(&count).Error; err != nil {
			return err
		}
		if count >= 100 {
			return access.ErrQuota
		}
		token, hash, err := access.NewToken()
		if err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO access_share_sessions(token_hash,share_id,expires_at,created_at) VALUES (?,?,?,?)`, hash, id, row.ExpiresAt, now).Error; err != nil {
			return err
		}
		p, err := sharePrincipal(tx, row)
		if err != nil {
			return err
		}
		result = access.ShareSession{Token: token, ExpiresAt: row.ExpiresAt, Principal: p}
		return nil
	})
	return result, err
}

func (s *Store) AuthenticateShareSession(ctx context.Context, raw string, now time.Time) (access.Principal, error) {
	if access.TokenHash(raw) == "" {
		return access.Principal{}, access.ErrUnauthorized
	}
	db := s.db.WithContext(ctx)
	row, err := validShare(db, `access_shares.expires_at>? AND EXISTS(SELECT 1 FROM access_share_sessions
		WHERE share_id=access_shares.id AND token_hash=? AND expires_at>?)`, now, access.TokenHash(raw), now)
	if err != nil {
		return access.Principal{}, err
	}
	return sharePrincipal(db, row)
}

func (s *Store) ValidateDemo(ctx context.Context, p access.Principal) error {
	if p.Kind != access.Demo || p.TenantCode == "" || len(p.AllowedHosts) == 0 {
		return access.ErrForbidden
	}
	var active bool
	if err := s.db.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM tenants WHERE tenant_code=? AND status=1)`, p.TenantCode).Scan(&active).Error; err != nil {
		return err
	}
	if !active {
		return access.ErrForbidden
	}
	return nil
}
