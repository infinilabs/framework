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

package config

type OAuthConfig struct {
	Enabled             bool                `config:"enabled" json:"enabled"`
	Name                string              `config:"name" json:"name"`
	Provider            string              `config:"provider" json:"provider"`
	ClientID            string              `config:"client_id" json:"client_id"`
	ClientSecret        string              `config:"client_secret" json:"client_secret"`
	DefaultRoles        []string            `config:"default_roles" json:"default_roles"`
	RoleMapping         map[string][]string `config:"role_mapping" json:"role_mapping"`
	AuthorizeUrl        string              `config:"authorize_url" json:"authorize_url"`
	TokenUrl            string              `config:"token_url" json:"token_url"`
	RedirectUrl         string              `config:"redirect_url" json:"redirect_url"`
	ProfileUrl          string              `config:"profile_url" json:"profile_url"`
	Scopes              []string            `config:"scopes" json:"scopes"`
	BootstrapAdminUsers []string            `config:"bootstrap_admin_users" json:"bootstrap_admin_users"`

	SuccessPage string `config:"success_page" json:"success_page"`
	FailedPage  string `config:"failed_page" json:"failed_page"`
}
