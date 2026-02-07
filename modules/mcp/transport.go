// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

// MaxRequestBodySize limits the size of incoming MCP requests.
const MaxRequestBodySize = 1024 * 1024 // 1 MB

// ServeHTTP handles an MCP HTTP request.
// Supports both POST (single JSON-RPC request) and GET (SSE streaming).
func ServeHTTP(w http.ResponseWriter, r *http.Request, toolCtx *ToolContext) {
	// Handle CORS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		serveSSE(w, r, toolCtx)
	case http.MethodPost:
		handlePost(w, r, toolCtx)
	default:
		http.Error(w, "Method not allowed. Use GET for SSE or POST for single requests.", http.StatusMethodNotAllowed)
	}
}

// handlePost processes a single POST JSON-RPC request.
func handlePost(w http.ResponseWriter, r *http.Request, toolCtx *ToolContext) {
	// Set CORS headers for browser clients
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if this is a message to an SSE session
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		handleSessionMessage(w, r, sessionID)
		return
	}

	// Validate Content-Type
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Validate Accept header
	accept := r.Header.Get("Accept")
	if accept != "" && !strings.Contains(accept, "application/json") && !strings.Contains(accept, "*/*") {
		http.Error(w, "Accept must include application/json", http.StatusNotAcceptable)
		return
	}

	// Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBodySize))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse as a single JSON-RPC request (batch not supported for MVP)
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONResponse(w, jsonRPCError(nil, -32700, "Parse error: "+err.Error()))
		return
	}

	if req.JSONRPC != "2.0" {
		writeJSONResponse(w, jsonRPCError(req.ID, -32600, "Invalid JSON-RPC version"))
		return
	}

	resp := HandleJSONRPC(&req, toolCtx)

	// Notifications don't get a response
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	writeJSONResponse(w, resp)
}

// handleSessionMessage routes a POST with Mcp-Session-Id to the correct SSE session.
func handleSessionMessage(w http.ResponseWriter, r *http.Request, sessionID string) {
	session := sessionManager.Get(sessionID)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Validate Content-Type
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBodySize))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONResponse(w, jsonRPCError(nil, -32700, "Parse error: "+err.Error()))
		return
	}

	if req.JSONRPC != "2.0" {
		writeJSONResponse(w, jsonRPCError(req.ID, -32600, "Invalid JSON-RPC version"))
		return
	}

	// Send to session for processing
	if !session.SendRequest(&req) {
		http.Error(w, "Session closed", http.StatusGone)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func writeJSONResponse(w http.ResponseWriter, resp *JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("MCP: failed to marshal response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
