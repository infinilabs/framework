package api

import (
	"strings"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
)

func ValidateServerExposureConfig(cfg *config.SystemConfig) error {
	if cfg == nil {
		return nil
	}
	if cfg.APIConfig.Enabled && !cfg.APIConfig.Security.Enabled {
		return errors.Errorf("unsafe config: api.enabled requires api.security.enabled")
	}
	if cfg.WebAppConfig.Enabled && cfg.WebAppConfig.EmbeddingAPI {
		return errors.Errorf("unsafe config: web.embedding_api is forbidden; use protected UI routes instead")
	}
	if cfg.APIConfig.Security.Enabled {
		if strings.TrimSpace(cfg.APIConfig.Security.Username) == "" {
			return errors.Errorf("unsafe config: api.security.username is required when api.security.enabled is true")
		}
	}
	return nil
}
