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

// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/logging/logger"
	"infini.sh/framework/core/stats"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Hub maintains the set of active connections and broadcasts messages to the
// connections.
type Hub struct {
	// Registered connections.
	connections map[*WebsocketConnection]bool

	// Inbound messages from the connections.
	broadcast chan string

	// Register requests from the connections.
	register chan *WebsocketConnection

	// Unregister requests from connections.
	unregister chan *WebsocketConnection

	// Command handlers
	handlers map[string]WebsocketHandlerFunc

	// Connection sessions
	sessions map[string]*WebsocketConnection

	//Command usage tips
	usage map[string]string
}

// WebsocketHandlerFunc define the func to handle websocket
type WebsocketHandlerFunc func(c *WebsocketConnection, array []string)

var h = Hub{
	broadcast:   make(chan string, 5),
	register:    make(chan *WebsocketConnection),
	unregister:  make(chan *WebsocketConnection),
	connections: make(map[*WebsocketConnection]bool),
	sessions:    make(map[string]*WebsocketConnection),
	handlers:    make(map[string]WebsocketHandlerFunc),
	usage:       make(map[string]string),
}

var runningHub = false

// Register command handlers
func (h *Hub) registerHandlers() {
	HandleWebSocketCommand("HELP", "type `help` for more commands", helpCommand)
}

// InitWebSocket start websocket
func InitWebSocket(cfg config.WebsocketConfig) {
	if cfg.SkipHostVerify {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	} else {
		if len(cfg.PermittedHosts) > 0 {
			upgrader.CheckOrigin = func(r *http.Request) bool {
				origin := r.Header["Origin"]
				if len(origin) == 0 {
					return true
				}
				u, err := url.Parse(origin[0])
				if err != nil {
					return false
				}
				if strings.EqualFold(u.Host, r.Host) {
					return true
				}
				for _, oh := range cfg.PermittedHosts {
					if strings.EqualFold(u.Host, oh) {
						return true
					}
				}
				return false
			}

		}
	}
	if !runningHub {
		h.registerHandlers()
		runningHub = true
		go h.runHub()
	}

}

// HandleWebSocketCommand used to register command and handler
func HandleWebSocketCommand(cmd, usage string, handler func(c *WebsocketConnection, array []string)) {
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	h.handlers[cmd] = WebsocketHandlerFunc(handler)
	h.usage[cmd] = usage
}

func (h *Hub) runHub() {
	//TODO error　handler,　parameter　assertion

	if global.Env().IsDebug {

		go func() {
			t := time.NewTicker(time.Duration(30) * time.Second)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					h.broadcast <- "testing websocket broadcast"
				}
			}
		}()
	}

	//handle connect, disconnect, broadcast
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
			h.sessions[c.id] = c
			c.WritePrivateMessage(global.Env().GetWelcomeMessage())
			js, _ := json.Marshal(logger.GetLoggingConfig())
			c.WriteMessage(ConfigMessage, string(js))
			c.WriteMessage(ConfigMessage, "websocket-session-id: "+c.id)
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				delete(h.sessions, c.id)
				close(c.signalChannel)
			}
		case m := <-h.broadcast:
			h.broadcastMessage(m)
		}
	}

}

func (h *Hub) broadcastMessage(msg string) {

	if len(msg) <= 0 {
		return
	}

	for c := range h.connections {
		c.Broadcast(msg)
	}
}

// BroadcastMessage send broadcast message to channel and record stats
func BroadcastMessage(msg string) {
	select {
	case h.broadcast <- msg:
		stats.Increment("websocket", "sended")
	default:
		stats.Increment("websocket", "dropped")
	}
}

func SendPrivateMessage(session string, msg string) {
	if c, ok := h.sessions[session]; ok {
		c.WritePrivateMessage(msg)
	}
}

func getHelpMessage() string {
	//list all commands and usage
	help := "COMMAND LIST\n"
	for k, v := range h.usage {
		help += (k + ", " + v + "\n")
	}
	return help
}

// Help command returns command help information
func helpCommand(c *WebsocketConnection, a []string) {
	c.WritePrivateMessage(getHelpMessage())
}
