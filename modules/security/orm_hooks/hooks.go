package orm_hooks

import (
	"errors"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/security/share"
)

func init() {

	global.RegisterFuncAfterSetup(func() {

		//only for managed mode, then check for tenant and user, must have and permit
		if global.Env().SystemConfig.WebAppConfig.Security.Managed {
			log.Debug("skip init ORM hooks for none-managed mode")
			return
		}

		sharingService := share.NewSharingService()

		orm.RegisterDataOperationPreHook(10, func(ctx *orm.Context, op orm.Operation, o interface{}) (*orm.Context, interface{}, error) {

			if ctx == nil {
				log.Debug(op, ",", util.MustToJSON(o))
				panic("invalid data access")
			}

			if op == orm.OpGet && ctx.GetBool(orm.DirectReadWithoutPermissionCheck, false) {
				return ctx, o, nil
			}

			if op == orm.OpDelete && ctx.GetBool(orm.DirectWriteWithoutPermissionCheck, false) {
				return ctx, o, nil
			}

			sessionUser := security.MustGetUserFromContext(ctx.Context)
			userID := sessionUser.MustGetUserID()

			//bypass admin
			if util.ContainsAnyInArray(security.RoleAdmin, sessionUser.Roles) {
				return ctx, o, nil
			}

			v := o.(orm.SystemFieldAccessor)
			userID1 := v.GetSystemString(OwnerIDKey)

			//bypass owner
			if userID1 == userID {
				return ctx, o, nil
			}

			//bypass explicit shared item
			if ctx.GetBool(orm.SharingEnabled, false) {

				o1, ok := o.(orm.Object)
				if !ok || o1 == nil {
					panic("invalid object")
				}

				resourceType := ctx.MustGetString(orm.SharingResourceType)
				per, err := sharingService.GetUserExplicitEffectivePermission(userID, share.NewResourceEntity(resourceType, o1.GetID(), ""))
				if err == nil {
					log.Debug("get permission: ", resourceType, ",", o1.GetID(), " => ", per)
					if op == orm.OpGet && per >= 1 {
						return ctx, o, nil
					}
				}

				if ctx.GetBool(orm.SharingCategoryCheckingChildrenEnabled, false) {
					per, err := sharingService.GetCategoryVisibleWithChildrenSharedObjects(userID, resourceType, o1.GetID())
					if err == nil {
						if op == orm.OpGet && per >= 1 {
							log.Debugf("the resource is category,and children are with %v permission, allow to read access", per)
							return ctx, o, nil
						}
					}
				}
			}

			var invalid = false
			if userID1 != "" {
				if userID1 != userID {
					invalid = true
				}
			}

			if invalid {
				panic("invalid data access")
				log.Debug("invalid data access, user: ", userID, " vs ", userID1, ",", util.MustToJSON(o))
				return ctx, nil, errors.New("invalid data access")
			}

			//delete op must within user's permission
			if op == orm.OpDelete {
				if userID1 == "" {
					log.Debug("invalid data access, user: ", userID, " vs ", userID1, ",", util.MustToJSON(o))
					return ctx, nil, errors.New("invalid data access")
				}
			}

			return ctx, o, nil
		}, orm.OpGet, orm.OpDelete)

		orm.RegisterDataOperationPostHook(10, func(ctx *orm.Context, op orm.Operation, o interface{}) (*orm.Context, interface{}, error) {

			if ctx == nil {
				log.Debug(op, ",", util.MustToJSON(o))
				panic("invalid data access")
			}

			if ctx.GetBool(orm.DirectWriteWithoutPermissionCheck, false) {
				return ctx, o, nil
			}

			sessionUser := security.MustGetUserFromContext(ctx.Context)
			userID := sessionUser.MustGetUserID()

			//bypass admin
			if util.ContainsAnyInArray(security.RoleAdmin, sessionUser.Roles) {
				return ctx, o, nil
			}

			v := o.(orm.SystemFieldAccessor)
			userID1 := v.GetSystemString(OwnerIDKey)

			//bypass owner
			if userID1 == userID {
				return ctx, o, nil
			}

			o1, ok := o.(orm.Object)
			if !ok || o1 == nil {
				panic("invalid object")
			}

			//bypass explicit shared item
			if ctx.GetBool(orm.SharingEnabled, false) {
				resourceType := ctx.MustGetString(orm.SharingResourceType)

				shareEntity := share.NewResourceEntity(resourceType, o1.GetID(), "")
				if ctx.GetBool(orm.SharingCheckingResourceCategoryEnabled, false) {
					shareEntity.ResourceCategoryType = ctx.MustGetString(orm.SharingResourceCategoryType)
					shareEntity.ResourceCategoryID = ctx.MustGetString(orm.SharingResourceCategoryID)
					shareEntity.ResourceParentPath = ctx.MustGetString(orm.SharingResourceParentPath)
				}

				per, err := sharingService.GetUserExplicitEffectivePermission(userID, shareEntity)
				if err == nil {
					log.Debug("get permission: ", resourceType, ",", o1.GetID(), " => ", per)
					if op == orm.OpGet && per >= 1 {
						return ctx, o, nil
					}
				}

				if ctx.GetBool(orm.SharingCategoryCheckingChildrenEnabled, false) {
					per, err := sharingService.GetCategoryVisibleWithChildrenSharedObjects(userID, resourceType, o1.GetID())
					if err == nil {
						if op == orm.OpGet && per >= 1 {
							log.Debug("the resource is category,and children are with share permission, allow to read access")
							return ctx, o, nil
						}
					}
				}
			}

			var invalid = false
			if userID1 != "" {
				if userID1 != userID {
					invalid = true
				}
			}

			//must set and must equal
			if invalid {
				panic(errors.New("invalid data access"))
				log.Debug("invalid data access, user: ", userID, " vs ", userID1, ",", util.MustToJSON(o))
				return ctx, nil, errors.New("invalid data access")
			}

			if !ctx.GetBool(orm.KeepSystemFields, false) {
				//protect system field, don't output
				v.SetSystemValues(nil)
			}

			return ctx, v, nil

		}, orm.OpDelete)

		orm.RegisterDataOperationPreHook(10, func(ctx *orm.Context, op orm.Operation, o interface{}) (*orm.Context, interface{}, error) {

			if ctx == nil {
				log.Debug(op, ",", util.MustToJSON(o))
				panic("invalid data access")
			}

			optional:=false
			if ctx.GetBool(orm.DirectWriteWithoutPermissionCheck, false) {
				optional=true
			}

			sessionUser,err := security.GetUserFromContext(ctx.Context)
			if !optional &&(sessionUser==nil||err!=nil){
				panic("invalid user info")
			}

			if sessionUser!=nil{
				userID := sessionUser.MustGetUserID()

				v, ok := o.(orm.SystemFieldAccessor)
				if !ok {
					panic("object does not implement SystemAccessor")
				}
				v.SetSystemValue(OwnerIDKey, userID)
				return ctx, v, nil
			}

			return ctx, o, nil
		}, orm.OpCreate)

		orm.RegisterDataOperationPreHook(10, func(ctx *orm.Context, op orm.Operation, o interface{}) (*orm.Context, interface{}, error) {

			if ctx == nil {
				log.Debug(op, ",", util.MustToJSON(o))
				panic("invalid data access")
			}

			if ctx.GetBool(orm.DirectWriteWithoutPermissionCheck, false) {
				return ctx, o, nil
			}

			sessionUser := security.MustGetUserFromContext(ctx.Context)
			userID := sessionUser.MustGetUserID()

			//bypass admin
			if util.ContainsAnyInArray(security.RoleAdmin, sessionUser.Roles) {
				return ctx, o, nil
			}

			o1, ok := o.(orm.Object)
			if !ok || o1 == nil {
				panic("invalid object")
			}

			//bypass explicit shared item
			if ctx.GetBool(orm.SharingEnabled, false) {
				resourceType := ctx.MustGetString(orm.SharingResourceType)
				shareEntity := share.NewResourceEntity(resourceType, o1.GetID(), "")
				if ctx.GetBool(orm.SharingCheckingResourceCategoryEnabled, false) {
					shareEntity.ResourceCategoryType = ctx.MustGetString(orm.SharingResourceCategoryType)
					shareEntity.ResourceCategoryID = ctx.MustGetString(orm.SharingResourceCategoryID)
					shareEntity.ResourceParentPath = ctx.MustGetString(orm.SharingResourceParentPath)
				}
				per, err := sharingService.GetUserExplicitEffectivePermission(userID, shareEntity)
				if err == nil {
					log.Debug("get permission: ", resourceType, ",", o1.GetID(), " => ", per)
					if per >= 4 {
						return ctx, o, nil
					}
				}
			}

			//target object maybe stripped system info
			userID1 := orm.GetOwnerID(o) //not defined in object
			if userID1 == "" {
				if ctx.GetBool(orm.AssignToCurrentUserIfNotExists, true) {
					//if no system info was found, set to current user
					v, ok := o.(orm.SystemFieldAccessor)
					if !ok {
						panic("object does not implement SystemAccessor")
					}
					v.SetSystemValue(OwnerIDKey, userID)
				} else {
					log.Debug("missing tenant and user info")
					panic("missing tenant and user info")
				}
			} else {
				//TODO, can update other's data
				//check permission
				//TODO, if it is update, and is already set, the object maybe owned by someone else, we should not override the owner
				//should be same tenant id
				if userID1 != userID {
					log.Debug("invalid data access, user: ", userID, " vs ", userID1, ",", util.MustToJSON(o))
					return ctx, nil, errors.New("invalid data access")
				}
			}

			return ctx, o, nil
		}, orm.OpUpdate, orm.OpSave)

		orm.RegisterSearchOperationHook(10, func(ctx *orm.Context, op orm.Operation, qb *orm.QueryBuilder) error {
			if ctx == nil {
				log.Debug(op, ",", util.MustToJSON(qb))
				panic("invalid data access")
			}

			if ctx.GetBool(orm.DirectReadWithoutPermissionCheck, false) {
				return nil
			}

			sessionUser := security.MustGetUserFromContext(ctx.Context)
			userID := sessionUser.MustGetUserID()
			//bypass admin
			if util.ContainsAnyInArray(security.RoleAdmin, sessionUser.Roles) {
				return nil
			}

			var bq *orm.Clause = orm.ShouldQuery()

			var globalShareMustFilters = []*orm.Clause{}
			////apply sharing rules
			if ctx.GetBool(orm.SharingEnabled, false) {

				resourceType := ctx.MustGetString(orm.SharingResourceType)

				//TODO support multi category filter and bypass
				//check category level filter first!
				//apply parent sharing rules, like if the parent object is shared, eg: datasource level, all docs will be marked as shared
				if ctx.GetBool(orm.SharingCheckingResourceCategoryEnabled, false) {
					resourceCategoryType := ctx.MustGetString(orm.SharingResourceCategoryType)
					resourceCategoryID := ctx.MustGetString(orm.SharingResourceCategoryID)
					filterField := ctx.MustGetString(orm.SharingResourceCategoryFilterField)

					globalShareMustFilters = append(globalShareMustFilters, orm.TermQuery("resource_category_type", resourceCategoryType))
					globalShareMustFilters = append(globalShareMustFilters, orm.TermQuery("resource_category_id", resourceCategoryID))

					//check if the current user have access to this resource
					log.Trace("check if the current user have access to this resource")
					perm, err := sharingService.GetUserExplicitEffectivePermission(userID, share.NewResourceEntity(resourceCategoryType, resourceCategoryID, ""))
					log.Trace("user have access to this parent object", perm, err)
					if err == nil {
						//TODO, not right permission, just 403
						//self or not inherit any permission, we should throw a permission error
						if perm >= share.View {
							//bypassByCategoryFilter = true
							categoryFilter := orm.TermQuery(filterField, resourceCategoryID)
							bq.ShouldClauses = append(bq.ShouldClauses, categoryFilter)
						}
					}
				} else {
					//for none-documents search
					ids, err := sharingService.GetResourceIDsByResourceTypeAndUserID(userID, resourceType)
					log.Debug("user have access to this parent object", ids, err)
					if err == nil {
						//TODO, not permission, just 403
						//self or not inherit any permission, we should throw a permission error
						if len(ids) >= 1 {
							bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", ids))
						}
					}
				}

				//only enable this for documents search
				if ctx.GetBool(orm.SharingCheckingInheritedRulesEnabled, false) {
					//if not hit, then we try specify docs or specify folders
					resourceParentPath, _ := ctx.GetString(orm.SharingResourceParentPath)
					if resourceParentPath != "" {
						//we are search files in specify folder/path
						//check if the current user have access to this filtered path
						var rules []share.SharingRecord
						rules, _ = share.GetSharingRules(security.PrincipalTypeUser, userID, resourceType, "", resourceParentPath, globalShareMustFilters)
						log.Trace("get all shared rules: ",resourceParentPath,",type:", resourceType, " => ", util.MustToJSON(rules))

						if len(rules) > 0 {
							allowedIDs := []string{}
							deniedIDs := []string{}

							for _, v := range rules {
								switch {
								// ✅ Allow rules
								case v.Permission > share.None:
									allowedIDs = append(allowedIDs, v.ResourceID)

								// ❌ Deny rules
								case v.Permission == share.None:
									deniedIDs = append(deniedIDs, v.ResourceID)
								default:
									log.Error("invalid permission rule: ", util.ToJson(v, true))
								}
							}

							inheritedRule := share.BuildEffectiveInheritedRules(rules, resourceParentPath, true)

							if inheritedRule != nil {

								// --- CASE 1: user has folder-level access ---
								if inheritedRule.Permission > share.None {
									mustClauses := []*orm.Clause{}
									mustNotClauses := []*orm.Clause{}

									// ✅ Include all documents under the current folder
									mustClauses = append(mustClauses, orm.PrefixQuery("_system.parent_path", resourceParentPath))

									// ❌ Exclude explicitly denied items
									if len(deniedIDs) > 0 {
										mustNotClauses = append(mustNotClauses, orm.TermsQuery("id", deniedIDs))
									}

									// ✅ Combine must and must_not
									finalBool := orm.BooleanQuery()
									finalBool.MustClauses = append(finalBool.MustClauses, mustClauses...)
									finalBool.MustNotClauses = append(finalBool.MustNotClauses, mustNotClauses...)

									// Attach to main query
									bq.ShouldClauses = append(bq.ShouldClauses, finalBool)

									// ✅ Always include explicitly allowed IDs directly (e.g., shared items)
									if len(allowedIDs) > 0 {
										bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", allowedIDs))
									}

								} else {
									// Only include items explicitly shared to the user
									if len(allowedIDs) > 0 {
										bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", allowedIDs))
									} else {
										//// User has no access at all to anything here
									}
								}
							} else {
								//// --- CASE 3: no inherited rule at all ---
								//log.Warn("⚠️ No permission rule found, fallback to explicit allowed IDs")
								if len(allowedIDs) > 0 {
									bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", allowedIDs))
								}
							}

							// 🔍 Final debug output
							log.Debug("Final built query: ", util.MustToJSON(bq))
						}
					} else {
						var rules []share.SharingRecord
						rules, _ = share.GetSharingRules(security.PrincipalTypeUser, userID, resourceType, "", resourceParentPath, globalShareMustFilters)
						if len(rules) > 0 {
							allowedIDs := []string{}
							allowedFolderPaths := []string{}
							deniedIDs := []string{}
							deniedFolderPaths := []string{}

							for _, v := range rules {
								switch {
								// ✅ Allow rules
								case v.Permission > share.None:
									allowedIDs = append(allowedIDs, v.ResourceID)
									if v.ResourceIsFolder {
										allowedFolderPaths = append(allowedFolderPaths, v.ResourceFullPath)
									}

								// ❌ Deny rules
								case v.Permission == share.None:
									if v.ResourceIsFolder {
										deniedFolderPaths = append(deniedFolderPaths, v.ResourceFullPath)
									} else {
										deniedIDs = append(deniedIDs, v.ResourceID)
									}

								default:
									log.Error("invalid permission rule: ", util.ToJson(v, true))
								}
							}

							// --- Build final boolean query ---
							// ✅ allow items or folders
							shouldClauses := []*orm.Clause{}
							if len(allowedIDs) > 0 {
								shouldClauses = append(shouldClauses, orm.TermsQuery("id", allowedIDs))
							}
							for _, path := range allowedFolderPaths {
								shouldClauses = append(shouldClauses, orm.PrefixQuery("_system.parent_path", path))
							}
							if len(shouldClauses) > 0 {
								bq.ShouldClauses = append(bq.ShouldClauses, shouldClauses...)
							}

							// ❌ deny rules
							mustNotClauses := []*orm.Clause{}
							if len(deniedIDs) > 0 {
								mustNotClauses = append(mustNotClauses, orm.TermsQuery("id", deniedIDs))
							}
							for _, path := range deniedFolderPaths {
								// exclude docs under this path except explicitly allowed IDs
								folderExclude := orm.BooleanQuery()
								folderExclude.MustClauses = append(folderExclude.MustClauses,
									orm.PrefixQuery("_system.parent_path", path))
								if len(allowedIDs) > 0 {
									folderExclude.MustNotClauses = append(folderExclude.MustNotClauses,
										orm.TermsQuery("id", allowedIDs))
								}
								mustNotClauses = append(mustNotClauses, folderExclude)
							}

							if len(mustNotClauses) > 0 {
								bq.MustNotClauses = append(bq.MustNotClauses, mustNotClauses...)
							}
						}
					}

				}

				//this is a category, filter out by child shared rules
				//TODO, check specify datasource, if they are have access or not
				if ctx.GetBool(orm.SharingCategoryCheckingChildrenEnabled, false) {
					//eg: get datasource list by find out which doc was shared to you
					log.Debug("this is a category, filter out by child shared rules")
					vids, _ := sharingService.GetCategoryObjectFromSharedObjects(userID, resourceType)
					log.Trace("get shared ids via children: ", resourceType, " => ", vids)
					if len(vids) > 0 {
						bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", vids))
					}
				}
			}

			bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery(SystemOwnerQueryField, userID))

			if len(bq.ShouldClauses) > 1 {
				bq.Parameter("minimum_should_match", 1)
			}

			if bq != nil {
				qb.Must(bq)
			} else {
				qb.Filter(orm.MustQuery(orm.TermQuery(SystemOwnerQueryField, userID)))
			}

			return nil
		}, orm.OpSearch)

		orm.RegisterSearchOperationHook(10, func(ctx *orm.Context, op orm.Operation, qb *orm.QueryBuilder) error {
			if ctx.GetBool(orm.DirectWriteWithoutPermissionCheck, false) {
				return nil
			}

			if ctx != nil {
				sessionUser := security.MustGetUserFromContext(ctx.Context)
				userID := sessionUser.MustGetUserID()
				//bypass admin
				if util.ContainsAnyInArray(security.RoleAdmin, sessionUser.Roles) {
					return nil
				}

				//support batch delete self's data only
				qb.Filter(orm.MustQuery(orm.TermQuery(SystemOwnerQueryField, userID)))

			}

			return nil
		}, orm.OpDeleteByQuery)

	})

}

const OwnerIDKey = "owner_id"
const SystemOwnerQueryField = "_system.owner_id"
