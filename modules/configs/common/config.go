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
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package common

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
)

type AgentConfig struct {
	Enabled      bool `config:"enabled"`
	Setup *SetupConfig `config:"setup"`
}

type SetupConfig struct {
	DownloadURL               string      `config:"download_url"`
	CACertFile                string      `config:"ca_cert"`
	CAKeyFile                 string      `config:"ca_key"`
	ConsoleEndpoint           string      `config:"console_endpoint"`
	Port                      string      `config:"port"`
}

func GetAgentConfig() *AgentConfig {
	agentCfg := &AgentConfig{
		Enabled: true,
		Setup: &SetupConfig{
			DownloadURL: "https://release.infinilabs.com/agent/stable",
		},
	}
	_, err := env.ParseConfig("agent", agentCfg )
	if err != nil {
		log.Debug("agent config not found: %v", err)
	}
	if agentCfg.Setup.CACertFile == "" && agentCfg.Setup.CAKeyFile == "" {
		agentCfg.Setup.CACertFile, agentCfg.Setup.CAKeyFile, err = GetOrInitDefaultCaCerts()
		if err != nil {
			log.Errorf("generate default ca certs error: %v", err)
		}
	}
	return agentCfg
}
