package websocket

import (
	"testing"

	"infini.sh/framework/core/config"
)

func TestResolveMaxMessageSize(t *testing.T) {
	testCases := []struct {
		name   string
		cfg    config.WebsocketConfig
		expect int64
	}{
		{
			name:   "default",
			cfg:    config.WebsocketConfig{},
			expect: defaultMaxMessageSize,
		},
		{
			name: "custom",
			cfg: config.WebsocketConfig{
				MaxMessageSizeBytes: 1024,
			},
			expect: 1024,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if actual := resolveMaxMessageSize(tc.cfg); actual != tc.expect {
				t.Fatalf("unexpected websocket message limit: got %d want %d", actual, tc.expect)
			}
		})
	}
}
