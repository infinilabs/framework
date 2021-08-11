package pipeline

import (
	"fmt"
	log "github.com/cihub/seelog"
	 "infini.sh/framework/core/config"
	"infini.sh/framework/core/pipeline"
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
