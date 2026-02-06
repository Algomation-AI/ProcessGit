// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

const (
	// sseKeepaliveInterval is how often to send keepalive comments.
	sseKeepaliveInterval = 30 * time.Second

	// maxSessions limits the number of concurrent SSE sessions.
	maxSessions = 100

	// sessionRequestBuffer is the channel buffer size for incoming requests.
	sessionRequestBuffer = 16
)

// SSESession represents an active SSE connection with a client.
type SSESession struct {
	ID      string
	Writer  http.ResponseWriter
	Flusher http.Flusher
	ToolCtx *ToolContext
	reqCh   chan *JSONRPCRequest
	done    chan struct{}
	mu      sync.Mutex
	closed  bool
}

// SSESessionManager tracks active SSE sessions.
type SSESessionManager struct {
	sessions map[string]*SSESession
	mu       sync.RWMutex
}

// sessionManager is the global session registry.
var sessionManager = &SSESessionManager{
	sessions: make(map[string]*SSESession),
}

// Register adds a session to the manager. Returns false if at capacity.
func (m *SSESessionManager) Register(s *SSESession) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sessions) >= maxSessions {
		return false
	}
	m.sessions[s.ID] = s
	return true
}

// Unregister removes a session from the manager.
func (m *SSESessionManager) Unregister(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// Get retrieves a session by ID.
func (m *SSESessionManager) Get(id string) *SSESession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// SendRequest sends a JSON-RPC request to the session for processing.
// Returns false if the session is closed or the channel is full.
func (s *SSESession) SendRequest(req *JSONRPCRequest) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.reqCh <- req:
		return true
	default:
		return false
	}
}

// serveSSE handles a GET request to establish an SSE streaming connection.
func serveSSE(w http.ResponseWriter, r *http.Request, toolCtx *ToolContext) {
	// Validate Accept header
	accept := r.Header.Get("Accept")
	if accept != "" && !strings.Contains(accept, "text/event-stream") && !strings.Contains(accept, "*/*") {
		http.Error(w, "Accept header must include text/event-stream for SSE", http.StatusNotAcceptable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Generate session ID
	sessionID, err := generateSessionID()
	if err != nil {
		log.Error("MCP SSE: failed to generate session ID: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	session := &SSESession{
		ID:      sessionID,
		Writer:  w,
		Flusher: flusher,
		ToolCtx: toolCtx,
		reqCh:   make(chan *JSONRPCRequest, sessionRequestBuffer),
		done:    make(chan struct{}),
	}

	if !sessionManager.Register(session) {
		http.Error(w, "Too many active SSE sessions", http.StatusServiceUnavailable)
		return
	}
	defer func() {
		session.mu.Lock()
		session.closed = true
		session.mu.Unlock()
		sessionManager.Unregister(sessionID)
		log.Info("MCP SSE: session %s closed", sessionID)
	}()

	// Set SSE response headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id")
	w.Header().Set("Mcp-Session-Id", sessionID)

	log.Info("MCP SSE: session %s started for repo %d from %s", sessionID, toolCtx.RepoID, r.RemoteAddr)

	// Send the endpoint event so the client knows where to POST messages
	endpointURI := r.URL.Path
	if err := writeSSEEvent(w, flusher, "endpoint", endpointURI); err != nil {
		log.Error("MCP SSE: failed to send endpoint event: %v", err)
		return
	}

	// Event loop: process incoming requests and send keepalives
	ticker := time.NewTicker(sseKeepaliveInterval)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-session.reqCh:
			resp := HandleJSONRPC(req, toolCtx)
			if resp != nil {
				if err := writeSSEEvent(w, flusher, "message", resp); err != nil {
					log.Error("MCP SSE: failed to write response for session %s: %v", sessionID, err)
					return
				}
			}
		case <-ticker.C:
			if err := writeSSEComment(w, flusher, "keepalive"); err != nil {
				log.Error("MCP SSE: keepalive failed for session %s: %v", sessionID, err)
				return
			}
		}
	}
}

// writeSSEEvent writes a typed Server-Sent Event.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal SSE data: %w", err)
	}

	var buf strings.Builder
	if eventType != "" {
		fmt.Fprintf(&buf, "event: %s\n", eventType)
	}

	// SSE data lines (split on newlines for spec compliance)
	for _, line := range strings.Split(string(jsonData), "\n") {
		fmt.Fprintf(&buf, "data: %s\n", line)
	}
	buf.WriteString("\n")

	if _, err := w.Write([]byte(buf.String())); err != nil {
		return fmt.Errorf("write SSE event: %w", err)
	}
	flusher.Flush()
	return nil
}

// writeSSEComment writes an SSE comment line (used for keepalive).
func writeSSEComment(w http.ResponseWriter, flusher http.Flusher, comment string) error {
	if _, err := w.Write([]byte(": " + comment + "\n\n")); err != nil {
		return fmt.Errorf("write SSE comment: %w", err)
	}
	flusher.Flush()
	return nil
}

// generateSessionID creates a cryptographically random session identifier.
func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "mcp-" + hex.EncodeToString(b), nil
}
