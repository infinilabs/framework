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

package pipeline

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/config"
	"github.com/rubyniu105/framework/core/pipeline"
)

type EchoProcessor struct {
	cfg EchoConfig
}

func NewEchoProcessor(c *config.Config) (pipeline.Processor, error) {
	cfg := EchoConfig{}

	if err := c.Unpack(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of echo processor: %s", err)
	}

	return &EchoProcessor{
		cfg: cfg,
	}, nil

}

func (this *EchoProcessor) Name() string {
	return "echo"
}

type EchoConfig struct {
	Message string `config:"message"`
}

func (this *EchoProcessor) Process(c *pipeline.Context) error {
	log.Info("message:", this.cfg.Message)
	return nil
}
