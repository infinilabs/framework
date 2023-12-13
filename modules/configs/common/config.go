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
	Version                   string      `config:"version"`
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
			Version: "0.7.0-364",
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
