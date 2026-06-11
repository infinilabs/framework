package orm_hooks

import (
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

var auditSchemaRepairLocker sync.Mutex

func isMissingAuditSchemaError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "index_not_found_exception") ||
		strings.Contains(message, "resource_not_found_exception")
}

func saveAuditWithSchemaRepair(ctx *orm.Context, audit *event.Audit) error {
	err := orm.Save(ctx, audit)
	if !isMissingAuditSchemaError(err) {
		return err
	}

	auditSchemaRepairLocker.Lock()
	defer auditSchemaRepairLocker.Unlock()

	if initErr := orm.InitSchema(); initErr != nil {
		return initErr
	}

	return orm.Save(ctx, audit)
}

func init() {
	// Auto-audit: emit Audit events for all ORM write operations (Create, Update, Delete).
	// Runs at low priority (9999) so it executes AFTER all business hooks.
	orm.RegisterDataOperationPostHook(9999, func(ctx *orm.Context, op orm.Operation, o interface{}) (*orm.Context, interface{}, error) {
		if ctx == nil {
			return ctx, o, nil
		}
		// Skip internal/system writes that bypass permission checks (e.g., fixture seeding, migrations)
		if ctx.GetBool(orm.DirectWriteWithoutPermissionCheck, false) && ctx.GetBool(orm.DirectReadWithoutPermissionCheck, false) {
			return ctx, o, nil
		}

		var userID string
		sessionUser, _ := security.GetUserFromContext(ctx.Context)
		if sessionUser != nil {
			userID = sessionUser.MustGetUserID()
		}
		if userID == "" {
			return ctx, o, nil // no user context = system operation, skip audit
		}

		var resourceType, resourceID string
		if obj, ok := o.(orm.Object); ok {
			resourceID = obj.GetID()
		}
		resourceType = orm.GetIndexName(o)

		if resourceType == "" || resourceType == "audit-logs" {
			return ctx, o, nil // don't audit audit records themselves
		}

		var action string
		switch op {
		case orm.OpCreate:
			action = "orm.create"
		case orm.OpUpdate, orm.OpSave:
			action = "orm.update"
		case orm.OpDelete:
			action = "orm.delete"
		default:
			return ctx, o, nil
		}

		audit := &event.Audit{
			Timestamp: time.Now(),
			Metadata: event.AuditMetadata{
				Category: "data",
				Group:    resourceType,
				Action:   action,
				Outcome:  "success",
				UserID:   userID,
			},
			Fields: util.MapStr{
				"resource_type": resourceType,
				"resource_id":   resourceID,
			},
		}
		audit.SetID(util.GetUUID())
		audit.SetSystemValue(orm.OwnerIDKey, userID)

		auditCtx := orm.NewContext()
		auditCtx.DirectAccess()
		if err := saveAuditWithSchemaRepair(auditCtx, audit); err != nil {
			log.Warnf("failed to save auto-audit: %v", err)
		}

		return ctx, o, nil
	}, orm.OpCreate, orm.OpUpdate, orm.OpDelete, orm.OpSave)
}
