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

const (
	// None is for empty action
	None string = "none"
	// Create is for create action
	Create string = "create"
	// Read is for read action
	Read string = "read"
	// Update is for  update action
	Update string = "update"
	// Delete is for delete action
	Delete string = "delete"
	// Search is for search action
	Search string = "search"
	// CRUD is an alias for, create+read+update+delete permissions
	CRUD string = "crud"

	Admin string = "admin"
)
