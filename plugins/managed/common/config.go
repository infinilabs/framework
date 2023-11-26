/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package common

import (
	log "github.com/cihub/seelog"
	"infini.sh/cloud/modules/infra/runtime/model"
	"infini.sh/framework/core/env"
)


func GetAgentConfig() *model.AgentConfig {
	agentCfg := &model.AgentConfig{
		Enabled: true,
		Setup: &model.SetupConfig{
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