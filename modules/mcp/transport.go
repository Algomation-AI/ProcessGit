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
// It is called from the router handler with a pre-built ToolContext.
func ServeHTTP(w http.ResponseWriter, r *http.Request, toolCtx *ToolContext) {
	// Only POST is supported for MVP (no SSE GET streaming)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use POST.", http.StatusMethodNotAllowed)
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
