package common

import (
	"testing"

	"infini.sh/framework/core/config"
	coreenv "infini.sh/framework/core/env"
)

func TestParseAgentSetupReverseChannelEndpoints(t *testing.T) {
	cfg, err := config.NewConfigFrom(map[string]interface{}{
		"agent": map[string]interface{}{
			"enabled": true,
			"setup": map[string]interface{}{
				"console_endpoint":          "https://console.example.org",
				"reverse_channel_endpoints": []string{"wss://console.example.org/ws", "wss://console-backup.example.org/ws"},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	agentCfg := &AgentConfig{}
	exists, err := coreenv.ParseConfigSection(cfg, "agent", agentCfg)
	if err != nil {
		t.Fatalf("failed to parse agent config: %v", err)
	}
	if !exists {
		t.Fatal("expected agent config to exist")
	}
	if agentCfg.Setup == nil {
		t.Fatal("expected setup config to be parsed")
	}
	if len(agentCfg.Setup.ReverseChannelEndpoints) != 2 {
		t.Fatalf("expected 2 reverse channel endpoints, got %d", len(agentCfg.Setup.ReverseChannelEndpoints))
	}
	if agentCfg.Setup.ReverseChannelEndpoints[0] != "wss://console.example.org/ws" {
		t.Fatalf("unexpected first reverse channel endpoint: %q", agentCfg.Setup.ReverseChannelEndpoints[0])
	}
	if agentCfg.Setup.ReverseChannelEndpoints[1] != "wss://console-backup.example.org/ws" {
		t.Fatalf("unexpected second reverse channel endpoint: %q", agentCfg.Setup.ReverseChannelEndpoints[1])
	}
}
