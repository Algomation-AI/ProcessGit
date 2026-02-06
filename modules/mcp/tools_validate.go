// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import "fmt"

func toolValidate(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	var allErrors []string
	var allStats IndexStats
	allStats.TypeCounts = make(map[string]int)
	allValid := true

	for _, source := range ctx.Config.Sources {
		valid, errors, stats, err := ValidateXMLAgainstXSD(ctx.Commit, source)
		if err != nil {
			return &ToolCallResult{
				Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Validation error for %s: %s", source.Path, err.Error())}},
				IsError: true,
			}, nil
		}
		if !valid {
			allValid = false
		}
		allErrors = append(allErrors, errors...)
		allStats.TotalEntities += stats.TotalEntities
		for t, c := range stats.TypeCounts {
			allStats.TypeCounts[t] += c
		}
	}

	// Check for unique constraint violations
	nmrSeen := make(map[string]string)        // nmr -> entityID
	codeSeen := make(map[string]map[string]bool) // type -> set of codes
	for _, entity := range ctx.Index.Entities {
		// Check NMR uniqueness
		if nmr, ok := entity.Attributes["nmr"]; ok && nmr != "" {
			if existing, dup := nmrSeen[nmr]; dup {
				allErrors = append(allErrors, fmt.Sprintf("Duplicate NMR %s: %s and %s", nmr, existing, entity.ID))
				allValid = false
			}
			nmrSeen[nmr] = entity.ID
		}
		// Check code uniqueness within type
		if _, ok := codeSeen[entity.Type]; !ok {
			codeSeen[entity.Type] = make(map[string]bool)
		}
		code := entity.Attributes["code"]
		if code != "" {
			if codeSeen[entity.Type][code] {
				allErrors = append(allErrors, fmt.Sprintf("Duplicate %s code: %s", entity.Type, code))
				allValid = false
			}
			codeSeen[entity.Type][code] = true
		}
	}

	result := map[string]interface{}{
		"valid":  allValid,
		"errors": allErrors,
		"statistics": map[string]interface{}{
			"total_entities": allStats.TotalEntities,
			"by_type":        allStats.TypeCounts,
		},
	}

	if len(ctx.Config.Sources) > 0 {
		if schema := ctx.Config.Sources[0].Schema; schema != "" {
			result["schema"] = schema
		}
	}

	return jsonTextResult(result)
}
