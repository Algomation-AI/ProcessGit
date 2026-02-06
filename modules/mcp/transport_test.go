// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCancellableSSERequest creates a GET request with a cancellable context for SSE tests.
func newCancellableSSERequest(path string) (*http.Request, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	return req, cancel
}

// --- POST handler tests ---

func TestServeHTTP_PostInitialize(t *testing.T) {
	ctx := newTestToolContext()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	req := httptest.NewRequest(http.MethodPost, "/test/repo/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ServeHTTP(w, req, ctx)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp JSONRPCResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
}

func TestServeHTTP_PostPing(t *testing.T) {
	ctx := newTestToolContext()

	body := `{"jsonrpc":"2.0","id":2,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/test/repo/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ServeHTTP(w, req, ctx)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp JSONRPCResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	ctx := newTestToolContext()

	req := httptest.NewRequest(http.MethodPut, "/test/repo/mcp", nil)
	w := httptest.NewRecorder()

	ServeHTTP(w, req, ctx)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeHTTP_PostBadContentType(t *testing.T) {
	ctx := newTestToolContext()

	req := httptest.NewRequest(http.MethodPost, "/test/repo/mcp", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	ServeHTTP(w, req, ctx)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}

func TestServeHTTP_PostInvalidJSON(t *testing.T) {
	ctx := newTestToolContext()

	req := httptest.NewRequest(http.MethodPost, "/test/repo/mcp", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ServeHTTP(w, req, ctx)

	assert.Equal(t, http.StatusOK, w.Code) // JSON-RPC errors are returned as 200 with error payload

	var resp JSONRPCResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32700, resp.Error.Code)
}

func TestServeHTTP_PostNotification(t *testing.T) {
	ctx := newTestToolContext()

	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req := httptest.NewRequest(http.MethodPost, "/test/repo/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ServeHTTP(w, req, ctx)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

// --- SSE tests ---

func TestServeHTTP_SSEConnection(t *testing.T) {
	toolCtx := newTestToolContext()

	req, cancel := newCancellableSSERequest("/test/repo/mcp")
	defer cancel()
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeHTTP(w, req, toolCtx)
	}()

	// Give SSE handler time to write initial events
	time.Sleep(100 * time.Millisecond)

	// Check headers
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.NotEmpty(t, w.Header().Get("Mcp-Session-Id"))

	// Check that endpoint event was sent
	body := w.Body.String()
	assert.Contains(t, body, "event: endpoint")
	assert.Contains(t, body, "data: ")

	// Cancel and wait for goroutine to exit
	cancel()
	<-done
}

func TestServeHTTP_SSEBadAcceptHeader(t *testing.T) {
	ctx := newTestToolContext()

	req := httptest.NewRequest(http.MethodGet, "/test/repo/mcp", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	ServeHTTP(w, req, ctx)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestServeHTTP_SSEAcceptWildcard(t *testing.T) {
	toolCtx := newTestToolContext()

	req, cancel := newCancellableSSERequest("/test/repo/mcp")
	defer cancel()
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeHTTP(w, req, toolCtx)
	}()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))

	cancel()
	<-done
}

func TestServeHTTP_SSENoAcceptHeader(t *testing.T) {
	toolCtx := newTestToolContext()

	req, cancel := newCancellableSSERequest("/test/repo/mcp")
	defer cancel()
	// No Accept header — should be allowed (permissive)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ServeHTTP(w, req, toolCtx)
	}()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))

	cancel()
	<-done
}

// --- SSE session message tests ---

func TestHandleSessionMessage_SessionNotFound(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/test/repo/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", "nonexistent-session")
	w := httptest.NewRecorder()

	handleSessionMessage(w, req, "nonexistent-session")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- SSE helper function tests ---

func TestWriteSSEEvent(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  map[string]interface{}{},
	}

	err := writeSSEEvent(w, w, "message", data)
	require.NoError(t, err)

	body := w.Body.String()
	assert.Contains(t, body, "event: message\n")
	assert.Contains(t, body, "data: ")
	assert.True(t, strings.HasSuffix(body, "\n\n"))

	// Verify the data line is valid JSON
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			jsonPart := strings.TrimPrefix(line, "data: ")
			var parsed map[string]interface{}
			err := json.Unmarshal([]byte(jsonPart), &parsed)
			assert.NoError(t, err, "SSE data should be valid JSON")
		}
	}
}

func TestWriteSSEComment(t *testing.T) {
	w := httptest.NewRecorder()

	err := writeSSEComment(w, w, "keepalive")
	require.NoError(t, err)

	body := w.Body.String()
	assert.Equal(t, ": keepalive\n\n", body)
}

func TestWriteSSEEvent_EndpointType(t *testing.T) {
	w := httptest.NewRecorder()

	err := writeSSEEvent(w, w, "endpoint", "/test/repo/mcp")
	require.NoError(t, err)

	body := w.Body.String()
	assert.Contains(t, body, "event: endpoint\n")
	assert.Contains(t, body, `data: "/test/repo/mcp"`)
}

// --- Session manager tests ---

func TestSessionManager_RegisterAndGet(t *testing.T) {
	mgr := &SSESessionManager{sessions: make(map[string]*SSESession)}

	session := &SSESession{
		ID:    "test-session-1",
		reqCh: make(chan *JSONRPCRequest, 1),
		done:  make(chan struct{}),
	}

	ok := mgr.Register(session)
	assert.True(t, ok)

	got := mgr.Get("test-session-1")
	assert.NotNil(t, got)
	assert.Equal(t, "test-session-1", got.ID)

	got = mgr.Get("nonexistent")
	assert.Nil(t, got)
}

func TestSessionManager_Unregister(t *testing.T) {
	mgr := &SSESessionManager{sessions: make(map[string]*SSESession)}

	session := &SSESession{
		ID:    "test-session-2",
		reqCh: make(chan *JSONRPCRequest, 1),
		done:  make(chan struct{}),
	}

	mgr.Register(session)
	mgr.Unregister("test-session-2")

	got := mgr.Get("test-session-2")
	assert.Nil(t, got)
}

func TestSessionManager_MaxSessions(t *testing.T) {
	mgr := &SSESessionManager{sessions: make(map[string]*SSESession)}

	// Fill to capacity
	for i := 0; i < maxSessions; i++ {
		s := &SSESession{
			ID:    fmt.Sprintf("session-%d", i),
			reqCh: make(chan *JSONRPCRequest, 1),
			done:  make(chan struct{}),
		}
		ok := mgr.Register(s)
		assert.True(t, ok)
	}

	// Next one should fail
	s := &SSESession{
		ID:    "overflow",
		reqCh: make(chan *JSONRPCRequest, 1),
		done:  make(chan struct{}),
	}
	ok := mgr.Register(s)
	assert.False(t, ok)
}

func TestSSESession_SendRequest(t *testing.T) {
	session := &SSESession{
		ID:    "test-send",
		reqCh: make(chan *JSONRPCRequest, 1),
		done:  make(chan struct{}),
	}

	req := &JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "ping"}
	ok := session.SendRequest(req)
	assert.True(t, ok)

	// Read it back
	got := <-session.reqCh
	assert.Equal(t, "ping", got.Method)
}

func TestSSESession_SendRequestClosed(t *testing.T) {
	session := &SSESession{
		ID:     "test-closed",
		reqCh:  make(chan *JSONRPCRequest, 1),
		done:   make(chan struct{}),
		closed: true,
	}

	req := &JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "ping"}
	ok := session.SendRequest(req)
	assert.False(t, ok)
}

func TestGenerateSessionID(t *testing.T) {
	id1, err := generateSessionID()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(id1, "mcp-"))
	assert.Len(t, id1, 4+32) // "mcp-" + 32 hex chars

	id2, err := generateSessionID()
	require.NoError(t, err)
	assert.NotEqual(t, id1, id2, "Session IDs should be unique")
}

// --- Integration: POST to SSE session ---

func TestServeHTTP_PostToSSESession(t *testing.T) {
	toolCtx := newTestToolContext()

	// Start an SSE session with cancellable context
	sseReq, cancel := newCancellableSSERequest("/test/repo/mcp")
	defer cancel()
	sseReq.Header.Set("Accept", "text/event-stream")
	sseW := httptest.NewRecorder()

	sseDone := make(chan struct{})
	go func() {
		defer close(sseDone)
		ServeHTTP(sseW, sseReq, toolCtx)
	}()

	// Wait for SSE to establish
	time.Sleep(100 * time.Millisecond)

	// Get the session ID from headers
	sessionID := sseW.Header().Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID)

	// Send a ping via POST with session ID
	pingBody := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	postReq := httptest.NewRequest(http.MethodPost, "/test/repo/mcp", strings.NewReader(pingBody))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Mcp-Session-Id", sessionID)
	postW := httptest.NewRecorder()

	ServeHTTP(postW, postReq, toolCtx)

	assert.Equal(t, http.StatusAccepted, postW.Code)

	// Wait for the SSE handler to process the request
	time.Sleep(100 * time.Millisecond)

	// Check that the SSE stream contains the ping response
	sseBody := sseW.Body.String()
	assert.Contains(t, sseBody, "event: message")

	// Verify JSON-RPC response is in the SSE stream
	var found bool
	scanner := bufio.NewScanner(bytes.NewReader(sseW.Body.Bytes()))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var resp JSONRPCResponse
			if err := json.Unmarshal([]byte(data), &resp); err == nil {
				if resp.ID != nil && resp.Error == nil {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "SSE stream should contain a JSON-RPC response for the ping request")

	// Clean up SSE goroutine
	cancel()
	<-sseDone
}
