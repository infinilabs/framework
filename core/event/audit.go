// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package event

import (
	"time"

	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

func init() {
	orm.MustRegisterSchemaWithIndexName(Audit{}, "audit-logs")
}

// Audit represents an admin-facing security audit trail entry.
// Unlike Activity (which is user/app-facing), Audit records are for
// compliance, security review, and admin forensics.
// Embeds ORMObjectBase so _system.tenant_id and _system.owner_id are
// auto-wired by ORM hooks — no manual tenant stamping needed.
type Audit struct {
	orm.ORMObjectBase
	Timestamp time.Time     `json:"timestamp,omitempty" elastic_mapping:"timestamp:{type:date}"`
	Metadata  AuditMetadata `json:"metadata" elastic_mapping:"metadata:{type:object}"`
	Fields    util.MapStr   `json:"payload,omitempty" elastic_mapping:"payload:{type:object,enabled:false}"`
}

// AuditMetadata contains structured metadata for the audit event.
type AuditMetadata struct {
	// Category groups related audit events (e.g. "security", "data", "config")
	Category string `json:"category,omitempty" elastic_mapping:"category:{type:keyword}"`
	// Group is the sub-system (e.g. "sharing", "auth", "rbac", "orm")
	Group string `json:"group,omitempty" elastic_mapping:"group:{type:keyword}"`
	// Action is what happened (e.g. "share.grant", "share.revoke", "orm.create", "login")
	Action string `json:"action,omitempty" elastic_mapping:"action:{type:keyword}"`
	// Outcome: "success", "denied", "error"
	Outcome string `json:"outcome,omitempty" elastic_mapping:"outcome:{type:keyword}"`
	// UserID is the actor who performed the action
	UserID string `json:"user_id,omitempty" elastic_mapping:"user_id:{type:keyword}"`
	// Labels for additional indexed metadata
	Labels util.MapStr `json:"labels,omitempty" elastic_mapping:"labels:{type:object}"`
}
