// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package diagrams

import (
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/json"
)

const (
	DiagramBPMN    DiagramType = "bpmn"
	DiagramCMMN    DiagramType = "cmmn"
	DiagramDMN     DiagramType = "dmn"
	DiagramNGraph  DiagramType = "ngraph"
	DiagramRuleset DiagramType = "ruleset"
	DiagramNone    DiagramType = "none"
)

type DiagramType string

type DetectionResult struct {
	Type   DiagramType
	Format string
}

type rulesetMetadata struct {
	Source string `json:"source"`
	Type   string `json:"type"`
}

func (d DiagramType) Editable() bool {
	switch d {
	case DiagramBPMN, DiagramCMMN, DiagramDMN:
		return true
	default:
		return false
	}
}

func Detect(treePath string, headBytes []byte) DetectionResult {
	pathLower := strings.ToLower(treePath)
	if typ, format := detectByExtension(pathLower); typ != DiagramNone {
		return DetectionResult{Type: typ, Format: format}
	}

	typ, format := detectByContent(pathLower, headBytes)
	if format == "" {
		format = defaultFormatForType(typ)
	}
	return DetectionResult{Type: typ, Format: format}
}

func detectByExtension(pathLower string) (DiagramType, string) {
	switch {
	case strings.HasSuffix(pathLower, ".bpmn"), strings.HasSuffix(pathLower, ".bpmn20.xml"), strings.HasSuffix(pathLower, "bpmn.xml"):
		return DiagramBPMN, "xml"
	case strings.HasSuffix(pathLower, ".cmmn"), strings.HasSuffix(pathLower, ".cmmn11.xml"), strings.HasSuffix(pathLower, "cmmn.xml"):
		return DiagramCMMN, "xml"
	case strings.HasSuffix(pathLower, ".dmn"), strings.HasSuffix(pathLower, ".dmn11.xml"), strings.HasSuffix(pathLower, "dmn.xml"):
		return DiagramDMN, "xml"
	case strings.HasSuffix(pathLower, ".ngraph.json"):
		return DiagramNGraph, "json"
	case strings.HasSuffix(pathLower, ".ngraph.xml"):
		return DiagramNGraph, "xml"
	case strings.HasSuffix(pathLower, ".ngraph"):
		return DiagramNGraph, ""
	case strings.HasSuffix(pathLower, ".ruleset.json"):
		return DiagramRuleset, "json"
	case strings.HasSuffix(pathLower, ".ruleset.dmn"):
		return DiagramRuleset, "xml"
	case strings.HasSuffix(pathLower, ".ruleset"):
		return DiagramRuleset, ""
	default:
		return DiagramNone, ""
	}
}

func detectByContent(pathLower string, headBytes []byte) (DiagramType, string) {
	sample := strings.ToLower(string(headBytes))
	if len(sample) > 4096 {
		sample = sample[:4096]
	}

	switch {
	case strings.Contains(sample, "<bpmn:definitions") || strings.Contains(sample, "xmlns:bpmn="):
		return DiagramBPMN, "xml"
	case strings.Contains(sample, "<cmmn:definitions") || strings.Contains(sample, "xmlns:cmmn="):
		return DiagramCMMN, "xml"
	case strings.Contains(sample, "<dmn:definitions") || strings.Contains(sample, "xmlns:dmn="):
		return DiagramDMN, "xml"
	}

	if strings.HasSuffix(pathLower, ".json") || strings.HasPrefix(strings.TrimSpace(sample), "{") {
		if typ := detectDiagramJSON(headBytes); typ != DiagramNone {
			return typ, "json"
		}
	}

	return DiagramNone, ""
}

func detectDiagramJSON(headBytes []byte) DiagramType {
	if len(headBytes) > 4096 {
		headBytes = headBytes[:4096]
	}
	if len(strings.TrimSpace(string(headBytes))) == 0 {
		return DiagramNone
	}

	var meta map[string]any
	if err := json.Unmarshal(headBytes, &meta); err != nil {
		return DiagramNone
	}

	typeValue, _ := meta["type"].(string)
	switch strings.ToLower(typeValue) {
	case string(DiagramNGraph):
		return DiagramNGraph
	case string(DiagramRuleset):
		return DiagramRuleset
	}

	graphVal, hasGraph := meta["graph"].(map[string]any)
	hasNodes := false
	hasEdges := false

	if nodesAny, ok := meta["nodes"].([]any); ok && len(nodesAny) > 0 {
		hasNodes = true
	}
	if edgesAny, ok := meta["edges"].([]any); ok && len(edgesAny) > 0 {
		hasEdges = true
	}
	if hasGraph {
		if nodesAny, ok := graphVal["nodes"].([]any); ok && len(nodesAny) > 0 {
			hasNodes = true
		}
		if edgesAny, ok := graphVal["edges"].([]any); ok && len(edgesAny) > 0 {
			hasEdges = true
		}
	}
	if hasNodes && hasEdges {
		return DiagramNGraph
	}

	if _, ok := meta["rules"]; ok {
		return DiagramRuleset
	}
	if _, ok := meta["decisions"]; ok {
		return DiagramRuleset
	}
	return DiagramNone
}

func defaultFormatForType(diagramType DiagramType) string {
	switch diagramType {
	case DiagramBPMN, DiagramCMMN, DiagramDMN:
		return "xml"
	case DiagramNGraph, DiagramRuleset:
		if diagramType != DiagramNone {
			return "json"
		}
	}
	return ""
}

func CleanSourcePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if filepath.IsAbs(raw) {
		return ""
	}

	cleaned := filepath.Clean(raw)
	if strings.HasPrefix(cleaned, "..") {
		return ""
	}
	return cleaned
}

func ParseRulesetMetadata(data []byte) string {
	var meta rulesetMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return ""
	}
	return CleanSourcePath(meta.Source)
}
