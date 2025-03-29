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
	log "github.com/cihub/seelog"
	"github.com/gorilla/websocket"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 2 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebsocketConnection is an middleman between the websocket connection and the hub.
type WebsocketConnection struct {
	id string

	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	signalChannel chan []byte

	handlers map[string]WebsocketHandlerFunc
}

// readPump pumps messages from the websocket connection to the hub.
func (c *WebsocketConnection) readPump() {
	defer func() {
		h.unregister <- c
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Debugf("error: %v", err)
			}
			break
		}
		c.parseMessage(message)
	}
}

var l sync.Mutex

// write writes a message with the given message type and payload.
func (c *WebsocketConnection) internalWrite(mt int, payload []byte) error {
	l.Lock()
	defer l.Unlock()
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *WebsocketConnection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case message, ok := <-c.signalChannel:
			if !ok {
				c.internalWrite(websocket.CloseMessage, []byte{})
				return
			}

			c.parseMessage(message)

		case <-ticker.C:
			if err := c.internalWrite(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// MsgType is the type of different message
type MsgType string

const (
	// PrivateMessage means message is 1 to 1
	PrivateMessage MsgType = "PRIVATE"
	// PublicMessage means broadcast message
	PublicMessage MsgType = "PUBLIC"
	// ConfigMessage used to send configuration
	ConfigMessage MsgType = "CONFIG"

	// SystemMessage used to send system related info
	SystemMessage MsgType = "SYSTEM"
)

// WritePrivateMessage will send msg to channel
func (c *WebsocketConnection) WritePrivateMessage(msg string) error {

	return c.WriteMessage(PrivateMessage, msg)
}

// WriteMessage will use the right way to write message, don't call c.write directly
func (c *WebsocketConnection) WriteMessage(t MsgType, msg string) error {

	msg = string(t) + " " + msg

	return c.internalWrite(websocket.TextMessage, []byte(msg))
}

// parse received message, pass to specify handler
func (c *WebsocketConnection) parseMessage(msg []byte) {
	message := string(msg)
	array := strings.Split(message, " ")
	if len(array) > 0 {
		cmd := strings.ToLower(strings.TrimSpace(array[0]))
		if c.handlers != nil {
			handler := c.handlers[cmd]
			if handler != nil {
				handler(c, array)
				return
			}
		}
	}

	if err := c.WritePrivateMessage(getHelpMessage()); err != nil {
		return
	}

}

type ConnectCallbackFunc func(sessionID string, w http.ResponseWriter, r *http.Request) error

var callbacksOnConnect = []ConnectCallbackFunc{}
var lock = sync.RWMutex{}

func RegisterConnectCallback(f ConnectCallbackFunc) {
	lock.Lock()
	defer lock.Unlock()
	callbacksOnConnect = append(callbacksOnConnect, f)
}

type DisconnectCallbackFunc func(sessionID string)

var callbacksOnDisconnect = []DisconnectCallbackFunc{}

func RegisterDisconnectCallback(f DisconnectCallbackFunc) {
	lock.Lock()
	defer lock.Unlock()
	callbacksOnDisconnect = append(callbacksOnDisconnect, f)
}

// ServeWs handles websocket requests from the peer.
func ServeWs(w http.ResponseWriter, r *http.Request) {

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err)
		return
	}

	c := &WebsocketConnection{id: util.GetUUID(), signalChannel: make(chan []byte, 256), ws: ws, handlers: h.handlers}
	if callbacksOnConnect != nil && len(callbacksOnConnect) > 0 {
		//lock.Lock()
		//defer lock.Unlock()
		//TODO handle panic in callback
		for _, v := range callbacksOnConnect {
			err := v(c.id, w, r)
			if err != nil {
				if global.Env().IsDebug {
					log.Error(err)
				}
				closeMessage := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error())
				if closeErr := ws.WriteMessage(websocket.CloseMessage, closeMessage); closeErr != nil {
					log.Error("Failed to send close message:", closeErr)
				}

				// Close the websocket connection
				ws.Close()
				return
			}
		}
	}

	h.register <- c
	go c.writePump()
	c.readPump()
}

// Broadcast public message to all channels
func (c *WebsocketConnection) Broadcast(msg string) {
	c.WriteMessage(PublicMessage, msg)
}
