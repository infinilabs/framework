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

// DenyRoleAssignment is the anti-privilege-escalation guard for role
// assignment: a caller may only grant roles it holds itself. Callers holding
// an admin role (see IsAdminUser) are unrestricted. It returns the subset of
// the requested roles the caller is not allowed to grant; an empty (non-nil)
// slice means every requested role is allowed, and a nil requested slice
// yields an empty slice.
func DenyRoleAssignment(callerRoles []string, requested []string) []string {
	if IsAdminUser(callerRoles) {
		return nil
	}
	held := make(map[string]bool, len(callerRoles))
	for _, r := range callerRoles {
		held[r] = true
	}
	denied := make([]string, 0)
	for _, r := range requested {
		if !held[r] {
			denied = append(denied, r)
		}
	}
	return denied
}
