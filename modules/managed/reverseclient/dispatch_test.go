/* Copyright © INFINI Ltd. All rights reserved. */

package reverse

import (
	"strings"
	"testing"
)

// dispatch mimics serve()'s frame dispatch; kept in sync manually.
func dispatch(frame string) (command, payload string) {
	parts := strings.SplitN(frame, " ", 2)
	if len(parts) != 2 {
		return "", ""
	}
	payload = parts[1]
	command = parts[0]
	if command == "PRIVATE" || command == "CONFIG" {
		if sub := strings.SplitN(payload, " ", 2); len(sub) == 2 && strings.HasPrefix(sub[0], "reverse_") {
			command, payload = sub[0], sub[1]
		}
	}
	return command, payload
}

func TestFrameDispatch(t *testing.T) {
	cases := []struct {
		frame   string
		command string
		payload string
	}{
		{"CONFIG websocket-session-id: abc", "CONFIG", "websocket-session-id: abc"},
		{`PRIVATE reverse_request {"id":"1"}`, "reverse_request", `{"id":"1"}`},
		{"reverse_request {}", "reverse_request", "{}"},
		{"PRIVATE something_else x", "PRIVATE", "something_else x"},
		{"", "", ""},
	}
	for _, c := range cases {
		command, payload := dispatch(c.frame)
		if command != c.command {
			t.Errorf("frame %q: command = %q, want %q", c.frame, command, c.command)
		}
		if payload != c.payload {
			t.Errorf("frame %q: payload = %q, want %q", c.frame, payload, c.payload)
		}
	}
}
