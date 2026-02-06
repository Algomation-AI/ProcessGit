// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
)

// indexCache caches EntityIndex per repo+commit to avoid re-parsing.
var indexCache = struct {
	sync.RWMutex
	entries map[string]*EntityIndex
}{
	entries: make(map[string]*EntityIndex),
}

// GetOrBuildIndex returns a cached index or builds a new one.
func GetOrBuildIndex(repoID int64, commit *git.Commit, cfg *MCPConfig) (*EntityIndex, error) {
	cacheKey := fmt.Sprintf("%d:%s", repoID, commit.ID.String())

	indexCache.RLock()
	if idx, ok := indexCache.entries[cacheKey]; ok {
		indexCache.RUnlock()
		return idx, nil
	}
	indexCache.RUnlock()

	// Build index from all sources
	merged := &EntityIndex{
		Entities:  make(map[string]*Entity),
		ByType:    make(map[string][]string),
		ByParent:  make(map[string][]string),
		CommitSHA: commit.ID.String(),
		Stats:     IndexStats{TypeCounts: make(map[string]int)},
	}

	for _, source := range cfg.Sources {
		switch source.Type {
		case "xml":
			idx, err := ParseXMLSource(commit, source)
			if err != nil {
				return nil, err
			}
			// Merge into combined index
			for id, entity := range idx.Entities {
				merged.Entities[id] = entity
				merged.ByType[entity.Type] = append(merged.ByType[entity.Type], id)
				if entity.ParentID != "" {
					merged.ByParent[entity.ParentID] = append(merged.ByParent[entity.ParentID], id)
				}
			}
			merged.Stats.TotalEntities += idx.Stats.TotalEntities
			for t, c := range idx.Stats.TypeCounts {
				merged.Stats.TypeCounts[t] += c
			}
			if merged.SourceFile == "" {
				merged.SourceFile = source.Path
			}
		}
	}

	indexCache.Lock()
	// Simple cache eviction: keep max 100 entries
	if len(indexCache.entries) > 100 {
		indexCache.entries = make(map[string]*EntityIndex)
	}
	indexCache.entries[cacheKey] = merged
	indexCache.Unlock()

	return merged, nil
}

// SearchEntities performs a case-insensitive search across entity names and attributes.
func (idx *EntityIndex) SearchEntities(query string, limit int) []*Entity {
	if limit <= 0 {
		limit = 25
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}

	var results []*Entity
	for _, entity := range idx.Entities {
		if matchesQuery(entity, query) {
			results = append(results, entity)
			if len(results) >= limit {
				break
			}
		}
	}
	return results
}

func matchesQuery(entity *Entity, query string) bool {
	if strings.Contains(strings.ToLower(entity.Name), query) {
		return true
	}
	for _, v := range entity.Attributes {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(entity.ID), query) {
		return true
	}
	return false
}
