// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

// ParseXMLSource reads an XML file from Git and builds an EntityIndex.
func ParseXMLSource(commit *git.Commit, source MCPSource) (*EntityIndex, error) {
	xmlData, err := ReadFileContent(commit, source.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot read source %s: %w", source.Path, err)
	}

	index := &EntityIndex{
		Entities:   make(map[string]*Entity),
		ByType:     make(map[string][]string),
		ByParent:   make(map[string][]string),
		SourceFile: source.Path,
		CommitSHA:  commit.ID.String(),
		Stats:      IndexStats{TypeCounts: make(map[string]int)},
	}

	if err := parseXMLEntities(xmlData, index); err != nil {
		return nil, err
	}

	return index, nil
}

// parseXMLEntities walks the XML tree and extracts entities.
// Heuristic: any element that has a "code" attribute is treated as an entity.
func parseXMLEntities(data []byte, index *EntityIndex) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	type stackFrame struct {
		name     string
		attrs    map[string]string
		text     string
		parentID string
		depth    int
	}

	var stack []*stackFrame
	var currentParentID string

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("XML parse error: %w", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			localName := t.Name.Local
			attrs := make(map[string]string)
			for _, a := range t.Attr {
				if a.Name.Space == "" || a.Name.Space == "xml" {
					attrs[a.Name.Local] = a.Value
				}
			}

			frame := &stackFrame{
				name:     localName,
				attrs:    attrs,
				parentID: currentParentID,
				depth:    len(stack),
			}
			stack = append(stack, frame)

			// Entity heuristic: has a "code" attribute
			if code, hasCode := attrs["code"]; hasCode {
				entityType := localName
				entityID := entityType + ":" + code
				entity := &Entity{
					ID:         entityID,
					Type:       entityType,
					ParentID:   currentParentID,
					Attributes: attrs,
				}

				// Set name from "name" attribute if present
				if name, hasName := attrs["name"]; hasName && name != "" {
					entity.Name = name
				}

				index.Entities[entityID] = entity
				index.ByType[entityType] = append(index.ByType[entityType], entityID)
				if currentParentID != "" {
					index.ByParent[currentParentID] = append(index.ByParent[currentParentID], entityID)
					if parentEntity, ok := index.Entities[currentParentID]; ok {
						parentEntity.Children = append(parentEntity.Children, entityID)
					}
				}
				index.Stats.TotalEntities++
				index.Stats.TypeCounts[entityType]++

				// This entity becomes the current parent for children
				currentParentID = entityID
			}

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if len(stack) > 0 && text != "" {
				stack[len(stack)-1].text = text
			}

		case xml.EndElement:
			if len(stack) > 0 {
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]

				// If this frame was an entity, restore parent context
				if _, hasCode := frame.attrs["code"]; hasCode {
					entityID := frame.name + ":" + frame.attrs["code"]
					if _, ok := index.Entities[entityID]; ok {
						currentParentID = frame.parentID
					}
				}

				// Check if this was a <n> (name) element inside an entity
				if frame.name == "n" && frame.text != "" && frame.parentID != "" {
					if parentEntity, ok := index.Entities[frame.parentID]; ok {
						if parentEntity.Name == "" {
							parentEntity.Name = frame.text
						}
					}
				}
			}
		}
	}

	return nil
}

// ReadFileContent reads raw file bytes from the Git commit.
func ReadFileContent(commit *git.Commit, path string) ([]byte, error) {
	entry, err := commit.GetTreeEntryByPath(path)
	if err != nil {
		return nil, err
	}
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// ValidateXMLAgainstXSD performs basic XML well-formedness check
// and structural validation. For MVP: validates XML is well-formed
// and collects statistics. Full XSD validation can be added later.
func ValidateXMLAgainstXSD(commit *git.Commit, source MCPSource) (bool, []string, IndexStats, error) {
	xmlData, err := ReadFileContent(commit, source.Path)
	if err != nil {
		return false, nil, IndexStats{}, fmt.Errorf("cannot read %s: %w", source.Path, err)
	}

	// Well-formedness check
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var errors []string
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("XML error: %s", err.Error()))
			break
		}
	}

	// Parse for statistics
	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}
	_ = parseXMLEntities(xmlData, index) // best-effort for stats

	valid := len(errors) == 0
	return valid, errors, index.Stats, nil
}
