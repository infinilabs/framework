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
	VersionNumber                  string `json:"number,omitempty" elastic_mapping:"number: { type: keyword }"`
	FrameworkBuildCommitHash       string `json:"framework_hash,omitempty" elastic_mapping:"framework_hash: { type: keyword }"`
	FrameworkVendorBuildCommitHash string `json:"vendor_hash,omitempty" elastic_mapping:"vendor_hash: { type: keyword }"`
	BuildCommitHash                string `json:"build_hash,omitempty" elastic_mapping:"build_hash: { type: keyword }"`
	// BuildDate/EolDate must map to keyword, never a dynamic date: binaries
	// built without version ldflags report placeholders ("N/A"), which a
	// date mapping rejects with document_parsing_exception — killing
	// instance heartbeats on indices whose mapping predates this pin.
	BuildDate   string `json:"build_date,omitempty" elastic_mapping:"build_date: { type: keyword }"`
	BuildNumber string `json:"build_number,omitempty" elastic_mapping:"build_number: { type: keyword }"`
	EolDate     string `json:"eol_date,omitempty" elastic_mapping:"eol_date: { type: keyword }"`
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
