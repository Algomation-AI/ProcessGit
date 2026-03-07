// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"
	"strings"
)

// toolCheckConformity cross-references process entities against a requirements source
// in the same repository. It searches for regulatory requirements relevant to the
// specified process or entity and returns a conformity assessment.
func toolCheckConformity(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	entityID, _ := args["entity_id"].(string)
	query, _ := args["query"].(string)

	if entityID == "" && query == "" {
		return &ToolCallResult{
			Content: []ToolContent{{Type: "text", Text: "Error: either 'entity_id' or 'query' parameter is required"}},
			IsError: true,
		}, nil
	}

	// Find the target entity (if entity_id provided)
	var targetEntity *Entity
	if entityID != "" {
		if e, ok := ctx.Index.Entities[entityID]; ok {
			targetEntity = e
		} else {
			return &ToolCallResult{
				Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Entity not found: %s", entityID)}},
				IsError: true,
			}, nil
		}
	}

	// Build the effective search query
	searchQuery := query
	if targetEntity != nil && searchQuery == "" {
		// Use entity name and type as the search basis
		searchQuery = targetEntity.Name
		if targetEntity.Type != "" {
			searchQuery = targetEntity.Type + " " + searchQuery
		}
	}

	// Search for requirement entities across all sources
	// Requirements are identified by type containing "requirement", "rule", "prasība", "noteikums"
	requirementTypes := []string{"requirement", "rule", "prasība", "prasiba", "noteikums", "nosacījums", "nosacijums", "regulation", "criteria", "kritērijs"}

	var matchedRequirements []conformityMatch
	var allRequirements []*Entity

	// Collect all requirement-type entities
	for _, entity := range ctx.Index.Entities {
		isRequirement := false
		etype := strings.ToLower(entity.Type)
		for _, rt := range requirementTypes {
			if strings.Contains(etype, rt) {
				isRequirement = true
				break
			}
		}
		// Also check for role=requirement in attributes
		if role, ok := entity.Attributes["role"]; ok && strings.Contains(strings.ToLower(role), "requirement") {
			isRequirement = true
		}
		// Check source description for "requirement" keyword
		if !isRequirement {
			for _, src := range ctx.Config.Sources {
				if strings.Contains(strings.ToLower(src.Description), "requirement") ||
					strings.Contains(strings.ToLower(src.Description), "prasīb") ||
					strings.Contains(strings.ToLower(src.Description), "regulat") {
					// This source is a requirements source — all its entities are requirements
					if strings.HasPrefix(entity.ID, strings.TrimSuffix(src.Path, ".xml")) ||
						entity.Attributes["source_file"] == src.Path {
						isRequirement = true
						break
					}
				}
			}
		}
		if isRequirement {
			allRequirements = append(allRequirements, entity)
		}
	}

	if len(allRequirements) == 0 {
		return jsonTextResult(map[string]interface{}{
			"status":  "no_requirements_found",
			"message": "No regulatory requirements found in this repository. To enable conformity checking, add a data source containing requirement entities (type containing 'requirement', 'rule', 'prasība', or 'noteikums').",
			"entity":  entityID,
			"query":   query,
		})
	}

	// Match requirements against the target entity or query
	searchTerms := strings.Fields(strings.ToLower(searchQuery))
	for _, req := range allRequirements {
		score := matchRequirement(req, searchTerms, targetEntity)
		if score > 0 {
			matchedRequirements = append(matchedRequirements, conformityMatch{
				RequirementID:   req.ID,
				RequirementName: req.Name,
				RelevanceScore:  score,
				Attributes:      req.Attributes,
			})
		}
	}

	// Sort by relevance (simple bubble sort for small datasets)
	for i := 0; i < len(matchedRequirements); i++ {
		for j := i + 1; j < len(matchedRequirements); j++ {
			if matchedRequirements[j].RelevanceScore > matchedRequirements[i].RelevanceScore {
				matchedRequirements[i], matchedRequirements[j] = matchedRequirements[j], matchedRequirements[i]
			}
		}
	}

	// Limit results
	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	if len(matchedRequirements) > limit {
		matchedRequirements = matchedRequirements[:limit]
	}

	// Build result
	result := map[string]interface{}{
		"status":                   "conformity_check_complete",
		"total_requirements":       len(allRequirements),
		"matched_requirements":     len(matchedRequirements),
		"requirements":             matchedRequirements,
	}

	if targetEntity != nil {
		result["entity_id"] = entityID
		result["entity_name"] = targetEntity.Name
		result["entity_type"] = targetEntity.Type
	}
	if query != "" {
		result["query"] = query
	}

	// Add a conformity summary hint for the LLM
	if len(matchedRequirements) > 0 {
		result["instruction"] = "Review each matched requirement against the entity or process. For each requirement, assess whether the entity/process satisfies it, partially satisfies it, or does not satisfy it. Provide specific reasoning citing both the requirement text and the entity attributes."
	}

	return jsonTextResult(result)
}

// conformityMatch represents a requirement matched to the query/entity.
type conformityMatch struct {
	RequirementID   string            `json:"requirement_id"`
	RequirementName string            `json:"requirement_name"`
	RelevanceScore  int               `json:"relevance_score"`
	Attributes      map[string]string `json:"attributes"`
}

// matchRequirement scores how relevant a requirement is to the search terms and target entity.
func matchRequirement(req *Entity, searchTerms []string, target *Entity) int {
	score := 0
	reqText := strings.ToLower(req.Name)
	for k, v := range req.Attributes {
		reqText += " " + strings.ToLower(k) + " " + strings.ToLower(v)
	}

	// Score based on search term matches
	for _, term := range searchTerms {
		if strings.Contains(reqText, term) {
			score += 2
		}
	}

	// If target entity exists, check for type/domain overlap
	if target != nil {
		targetText := strings.ToLower(target.Name)
		for _, v := range target.Attributes {
			targetText += " " + strings.ToLower(v)
		}

		// Check if requirement references the same domain/category
		if target.Type != "" && strings.Contains(reqText, strings.ToLower(target.Type)) {
			score += 3
		}

		// Check attribute value overlap
		for _, tv := range target.Attributes {
			tvLower := strings.ToLower(tv)
			if len(tvLower) > 3 && strings.Contains(reqText, tvLower) {
				score += 1
			}
		}
	}

	// Boost if the requirement has explicit legal references
	legalTerms := []string{"pants", "pant", "likums", "noteikum", "regula", "article", "section", "§"}
	for _, lt := range legalTerms {
		if strings.Contains(reqText, lt) {
			score += 1
			break
		}
	}

	return score
}
