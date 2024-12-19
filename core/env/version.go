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

package env

type Version struct {
	VersionNumber                  string `json:"number,omitempty"`
	FrameworkBuildCommitHash       string `json:"framework_hash,omitempty"`
	FrameworkVendorBuildCommitHash string `json:"vendor_hash,omitempty"`
	BuildCommitHash                string `json:"build_hash,omitempty"`
	BuildDate                      string `json:"build_date,omitempty"`
	BuildNumber                    string `json:"build_number,omitempty"`
	EolDate                        string `json:"eol_date,omitempty"`
}

func (env *Env) GetVersionInfo() Version {
	return Version{
		VersionNumber:                  env.version,
		FrameworkBuildCommitHash:       env.frameworkCommit,
		FrameworkVendorBuildCommitHash: env.frameworkVendorCommit,
		BuildCommitHash:                env.commit,
		BuildDate:                      env.buildDate,
		BuildNumber:                    env.buildNumber,
		EolDate:                        env.eolDate,
	}
}
