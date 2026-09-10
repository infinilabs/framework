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

package security

// Permission keys for the instance-level operational routes (stats, queue
// browsing, config/keystore reads, pipeline task ops) that are served on the
// web port behind login + RBAC. Managed clients mint their self API token
// with InstanceOpsPermissionKeys() so manager-originated calls (reverse
// channel loopback, direct endpoint access) pass the permission filter;
// human users obtain these keys through role assignment.

var (
	PermissionSystemStatsRead      = GetSimplePermission("generic", "system:stats", Read)
	PermissionSystemQueueRead      = GetSimplePermission("generic", "system:queue", Read)
	PermissionSystemConfigRead     = GetSimplePermission("generic", "system:config", Read)
	PermissionSystemKeystoreRead   = GetSimplePermission("generic", "system:keystore", Read)
	PermissionSystemKeystoreUpdate = GetSimplePermission("generic", "system:keystore", Update)
	PermissionSystemPipelineRead   = GetSimplePermission("generic", "system:pipeline", Read)
	PermissionSystemPipelineDelete = GetSimplePermission("generic", "system:pipeline", Delete)
	PermissionSystemLogRead        = GetSimplePermission("generic", "system:log", Read)
)

// InstanceOpsPermissionKeys returns every permission key an instance's self
// API token needs to serve manager-originated operational calls.
func InstanceOpsPermissionKeys() []PermissionKey {
	return []PermissionKey{
		PermissionSystemStatsRead,
		PermissionSystemQueueRead,
		PermissionSystemConfigRead,
		PermissionSystemKeystoreRead,
		PermissionSystemKeystoreUpdate,
		PermissionSystemPipelineRead,
		PermissionSystemPipelineDelete,
		PermissionSystemLogRead,
	}
}
