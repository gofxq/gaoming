package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gofxq/gaoming/pkg/access"
	"github.com/gofxq/gaoming/pkg/contracts"
	hostruntime "github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"gorm.io/gorm"
)

// Lock the actor row until the operation commits, so a concurrent role/status
// change cannot overtake the permission check.
func operationTenant(tx *gorm.DB, p access.Principal, admin bool) (int64, error) {
	if !p.IsUser() || (admin && !p.IsAdmin()) {
		return 0, access.ErrForbidden
	}
	var row struct {
		TenantID int64
		Role     string
	}
	err := tx.Raw(`SELECT users.tenant_id,users.role FROM users JOIN tenants ON tenants.id=users.tenant_id
		WHERE users.id=? AND tenants.tenant_code=? AND users.status='active' AND tenants.status=1
		FOR SHARE OF users,tenants`, p.UserID, p.TenantCode).Scan(&row).Error
	if err != nil {
		return 0, err
	}
	if row.TenantID == 0 || (row.Role != "admin" && row.Role != "member") || (admin && row.Role != "admin") {
		return 0, access.ErrForbidden
	}
	return row.TenantID, nil
}

func writeOperationAudit(tx *gorm.DB, tenantID int64, p access.Principal, action, resourceType, id string, now time.Time) error {
	return tx.Exec(`INSERT INTO audit_logs(tenant_id,actor,action,resource_type,resource_id,success,created_at) VALUES (?,?,?,?,?,true,?)`, tenantID, p.Actor(), action, resourceType, id, now).Error
}

func (s *Store) CreateMaintenance(ctx context.Context, p access.Principal, req contracts.CreateMaintenanceWindowRequest) (hostruntime.MaintenanceWindow, error) {
	var result hostruntime.MaintenanceWindow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenantID, err := operationTenant(tx, p, true)
		if err != nil {
			return err
		}
		var exists bool
		switch req.ScopeType {
		case "tenant":
			exists = req.ScopeRef == p.TenantCode
		case "host":
			err = tx.Raw(`SELECT EXISTS(SELECT 1 FROM hosts WHERE tenant_id=? AND host_uid=?)`, tenantID, req.ScopeRef).Scan(&exists).Error
		case "group":
			err = tx.Raw(`SELECT EXISTS(SELECT 1 FROM host_groups WHERE tenant_id=? AND group_code=?)`, tenantID, req.ScopeRef).Scan(&exists).Error
		default:
			return access.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !exists {
			return access.ErrNotFound
		}
		model := maintenanceWindowModel{TenantID: tenantID, Title: req.Title, ScopeType: req.ScopeType, ScopeRef: req.ScopeRef, StartAt: req.StartAt, EndAt: req.EndAt, CreatedBy: p.Actor(), Reason: emptyStringPtr(req.Reason), Enabled: true}
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		id := strconv.FormatInt(model.ID, 10)
		if err := writeOperationAudit(tx, tenantID, p, "maintenance.create", "maintenance", id, time.Now().UTC()); err != nil {
			return err
		}
		result = hostruntime.MaintenanceWindow{ID: id, Title: req.Title, ScopeType: req.ScopeType, ScopeRef: req.ScopeRef, StartAt: req.StartAt, EndAt: req.EndAt, CreatedBy: p.Actor(), Reason: req.Reason}
		return nil
	})
	return result, err
}

func (s *Store) AckAlert(ctx context.Context, p access.Principal, alertID string, now time.Time) error {
	id, err := strconv.ParseInt(alertID, 10, 64)
	if err != nil || id <= 0 {
		return access.ErrNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenantID, err := operationTenant(tx, p, false)
		if err != nil {
			return err
		}
		var event alertEventModel
		if err := tx.Raw(`SELECT * FROM alert_events WHERE id=? AND tenant_id=? FOR UPDATE`, id, tenantID).Take(&event).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return access.ErrNotFound
			}
			return err
		}
		if event.AckedAt != nil {
			return nil
		}
		if err := tx.Model(&event).Updates(map[string]any{"acked_by": p.Actor(), "acked_at": now}).Error; err != nil {
			return err
		}
		return writeOperationAudit(tx, tenantID, p, "alert.ack", "alert", alertID, now)
	})
}
