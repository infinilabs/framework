/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import "infini.sh/framework/core/param"

// Standard parameter keys for user session context.
// These keys are used with UserSessionInfo.GetString() / GetStringArray()
// since UserSessionInfo embeds param.Parameters.
const (
	ParamTenantID    param.ParaKey = "tenant_id"
	ParamTenantName  param.ParaKey = "tenant_name"
	ParamTeamID      param.ParaKey = "team_id"
	ParamTeamName    param.ParaKey = "team_name"
	ParamProjectID   param.ParaKey = "project_id"
	ParamProjectName param.ParaKey = "project_name"
	ParamTeamIDs     param.ParaKey = "team_ids"
	ParamProjectIDs  param.ParaKey = "project_ids"
)
