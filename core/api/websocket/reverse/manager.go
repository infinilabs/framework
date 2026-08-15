package reverse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"infini.sh/framework/core/util"
)

const (
	DefaultTimeout          = 30 * time.Second
	DefaultMaxResponseBytes = 8 * 1024 * 1024
	DefaultReconnectWait    = 6 * time.Second
	DefaultReconnectPoll    = 200 * time.Millisecond
)

var (
	ErrDisconnected = errors.New("reverse channel disconnected")
	ErrNotConnected = errors.New("reverse channel is not connected")
)

type ManagerOptions struct {
	DefaultTimeout   time.Duration
	MaxResponseBytes int
	ReconnectWait    time.Duration
	ReconnectPoll    time.Duration
}

type pendingResponse struct {
	peerID    string
	body      bytes.Buffer
	status    int
	err       error
	done      chan struct{}
	completed bool
}

type SessionManager struct {
	options            ManagerOptions
	mu                 sync.Mutex
	pendingSessions    map[string]string
	activeSessions     map[string]string
	activeSessionsByID map[string]string
	pendingResponses   map[string]*pendingResponse
}

func NewSessionManager(options ManagerOptions) *SessionManager {
	if options.DefaultTimeout <= 0 {
		options.DefaultTimeout = DefaultTimeout
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if options.ReconnectWait <= 0 {
		options.ReconnectWait = DefaultReconnectWait
	}
	if options.ReconnectPoll <= 0 {
		options.ReconnectPoll = DefaultReconnectPoll
	}
	return &SessionManager{
		options:            options,
		pendingSessions:    map[string]string{},
		activeSessions:     map[string]string{},
		activeSessionsByID: map[string]string{},
		pendingResponses:   map[string]*pendingResponse{},
	}
}

func (m *SessionManager) RegisterPendingSession(sessionID, peerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingSessions[sessionID] = strings.TrimSpace(peerID)
}

func (m *SessionManager) ActivateSession(sessionID, peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	peerID = strings.TrimSpace(peerID)
	if expectedPeerID, ok := m.pendingSessions[sessionID]; !ok || expectedPeerID != peerID {
		return fmt.Errorf("session handshake mismatch")
	}
	delete(m.pendingSessions, sessionID)

	if previousSession, ok := m.activeSessions[peerID]; ok && previousSession != sessionID {
		delete(m.activeSessionsByID, previousSession)
	}

	m.activeSessions[peerID] = sessionID
	m.activeSessionsByID[sessionID] = peerID
	return nil
}

func (m *SessionManager) HandleHelloPayload(payload string) error {
	msg, err := ParseHelloPayload(payload)
	if err != nil {
		return err
	}
	return m.ActivateSession(msg.SessionID, msg.PeerID)
}

func (m *SessionManager) HandleResponsePayload(payload string) error {
	msg, err := ParseResponsePayload(payload)
	if err != nil {
		return err
	}
	m.acceptResponse(msg)
	return nil
}

func (m *SessionManager) OnDisconnect(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.pendingSessions, sessionID)
	peerID, ok := m.activeSessionsByID[sessionID]
	if !ok {
		return
	}

	delete(m.activeSessionsByID, sessionID)
	if currentSession, exists := m.activeSessions[peerID]; exists && currentSession == sessionID {
		delete(m.activeSessions, peerID)
	}
	m.failPendingLocked(peerID, ErrDisconnected)
}

func (m *SessionManager) IsConnected(peerID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sessionID, ok := m.activeSessions[peerID]
	return ok && sessionID != ""
}

func (m *SessionManager) WaitForReconnect(ctx context.Context, peerID string) bool {
	waitCtx, cancel := context.WithTimeout(ctx, m.options.ReconnectWait)
	defer cancel()

	if m.IsConnected(peerID) {
		return true
	}

	ticker := time.NewTicker(m.options.ReconnectPoll)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return false
		case <-ticker.C:
			if m.IsConnected(peerID) {
				return true
			}
		}
	}
}

func IsRecoverableError(err error) bool {
	return errors.Is(err, ErrDisconnected) || errors.Is(err, ErrNotConnected)
}

func (m *SessionManager) ProxyRequest(peerID string, req *util.Request, headers http.Header, send func(sessionID, payload string) error, responseObjectToUnmarshal interface{}) (*util.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	ctx := req.Context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), m.options.DefaultTimeout)
		defer cancel()
	} else if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.options.DefaultTimeout)
		defer cancel()
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		res, err := m.proxyRequestOnce(ctx, strings.TrimSpace(peerID), req, headers, send, responseObjectToUnmarshal)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if attempt == 0 && IsRecoverableError(err) && m.WaitForReconnect(ctx, peerID) {
			continue
		}
		return res, err
	}
	return nil, lastErr
}

func (m *SessionManager) proxyRequestOnce(ctx context.Context, peerID string, req *util.Request, headers http.Header, send func(sessionID, payload string) error, responseObjectToUnmarshal interface{}) (*util.Result, error) {
	requestID := util.GetUUID()
	msg := RequestMessage{
		RequestID: requestID,
		PeerID:    peerID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   headers,
	}
	msg.SetBody(req.Body)
	if authorization := strings.TrimSpace(msg.Headers.Get("Authorization")); strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		msg.AccessToken = strings.TrimSpace(authorization[7:])
	}

	pending := &pendingResponse{
		peerID: peerID,
		done:   make(chan struct{}),
	}

	m.mu.Lock()
	sessionID, ok := m.activeSessions[peerID]
	if !ok || sessionID == "" {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w for peer [%s]", ErrNotConnected, peerID)
	}
	m.pendingResponses[requestID] = pending
	m.mu.Unlock()

	if err := send(sessionID, FormatRequestCommand(msg)); err != nil {
		m.mu.Lock()
		delete(m.pendingResponses, requestID)
		m.mu.Unlock()
		return nil, err
	}

	select {
	case <-pending.done:
	case <-ctx.Done():
		m.mu.Lock()
		delete(m.pendingResponses, requestID)
		m.mu.Unlock()
		return nil, ctx.Err()
	}

	if pending.err != nil {
		return nil, pending.err
	}

	res := &util.Result{
		Body:       pending.body.Bytes(),
		StatusCode: pending.status,
	}
	if res.StatusCode != http.StatusOK {
		return res, fmt.Errorf("request error: %s", string(res.Body))
	}
	if responseObjectToUnmarshal != nil && len(res.Body) > 0 {
		return res, util.FromJSONBytes(res.Body, responseObjectToUnmarshal)
	}
	return res, nil
}

func (m *SessionManager) acceptResponse(msg ResponseMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, ok := m.pendingResponses[msg.RequestID]
	if !ok || pending.completed {
		return
	}
	if msg.PeerID != "" && pending.peerID != "" && msg.PeerID != pending.peerID {
		return
	}

	if msg.Chunk != "" {
		chunk, err := msg.ChunkBytes()
		if err != nil {
			m.completePendingLocked(msg.RequestID, pending, 0, fmt.Errorf("decode reverse response chunk: %w", err))
			return
		}
		if pending.body.Len()+len(chunk) > m.options.MaxResponseBytes {
			m.completePendingLocked(msg.RequestID, pending, 0, fmt.Errorf("reverse response exceeds %d bytes", m.options.MaxResponseBytes))
			return
		}
		_, _ = pending.body.Write(chunk)
	}

	if msg.Done {
		status := msg.Status
		if status == 0 {
			status = http.StatusOK
		}
		m.completePendingLocked(msg.RequestID, pending, status, nil)
	}
}

func (m *SessionManager) completePendingLocked(requestID string, pending *pendingResponse, status int, err error) {
	if pending.completed {
		return
	}
	pending.completed = true
	pending.status = status
	pending.err = err
	close(pending.done)
	delete(m.pendingResponses, requestID)
}

func (m *SessionManager) failPendingLocked(peerID string, err error) {
	for requestID, pending := range m.pendingResponses {
		if pending.peerID != peerID {
			continue
		}
		m.completePendingLocked(requestID, pending, 0, err)
	}
}
