// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/logger"
	"infini.sh/framework/core/stats"
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
	handlers:    make(map[string]WebsocketHandlerFunc),
	usage:       make(map[string]string),
}

var runningHub = false

// Register command handlers
func (h *Hub) registerHandlers() {
	HandleWebSocketCommand("HELP", "type `help` for more commands", helpCommand)
}

// InitWebSocket start websocket
func InitWebSocket() {
	if !runningHub {
		h.registerHandlers()
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
			c.WritePrivateMessage(global.Env().GetWelcomeMessage())
			js, _ := json.Marshal(logger.GetLoggingConfig())
			c.WriteMessage(ConfigMessage, string(js))
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
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
