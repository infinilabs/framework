package orm_hooks

import (
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
)

func isAccessTokenModel(ctx *orm.Context) bool {
	if ctx == nil {
		return false
	}

	switch orm.GetModel(ctx).(type) {
	case *security.AccessToken, security.AccessToken:
		return true
	default:
		return false
	}
}

func init() {
	global.RegisterFuncAfterSetup(func() {
		// Access tokens are credential-like resources and must always be owner-isolated.
		// We enforce a strict owner filter here instead of relying on generic sharing rules.
		orm.RegisterSearchOperationHook(9, func(ctx *orm.Context, op orm.Operation, qb *orm.QueryBuilder) error {
			if ctx == nil || !isAccessTokenModel(ctx) {
				return nil
			}

			// Explicit internal bypass for trusted system flows only.
			if ctx.GetBool(orm.DirectReadWithoutPermissionCheck, false) {
				return nil
			}

			sessionUser := security.MustGetUserFromContext(ctx.Context)
			// Hard filter: only return tokens owned by the current user.
			// Keep this as a filter (AND) to prevent access via team/project/share paths.
			qb.Filter(orm.TermQuery(orm.GetSystemFieldKey(orm.OwnerIDKey), sessionUser.MustGetUserID()))
			return nil
		}, orm.OpSearch)
	})
}
