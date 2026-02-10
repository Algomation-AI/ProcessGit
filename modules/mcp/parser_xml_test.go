// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseXMLEntities(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<vdvcRegister xmlns="http://vdvc.gov.lv/schema/vdvc-register" version="1.0">
  <ministry code="01" name="Test Ministry One">
    <organization code="0001" nmr="90000038578" docPrefix="01-0001">
      <n>FIRST ORG</n>
    </organization>
  </ministry>
  <ministry code="02" name="">
    <organization code="0002" nmr="90000028300" docPrefix="02-0002">
      <n>SECOND ORG</n>
    </organization>
    <organization code="0003" nmr="90000055313" docPrefix="02-0003">
      <n>THIRD ORG</n>
    </organization>
  </ministry>
</vdvcRegister>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	// 2 ministries + 3 organizations = 5 entities
	assert.Equal(t, 5, index.Stats.TotalEntities)
	assert.Equal(t, 2, index.Stats.TypeCounts["ministry"])
	assert.Equal(t, 3, index.Stats.TypeCounts["organization"])

	// Check entity details
	org1 := index.Entities["organization:0001"]
	require.NotNil(t, org1)
	assert.Equal(t, "FIRST ORG", org1.Name)
	assert.Equal(t, "90000038578", org1.Attributes["nmr"])
	assert.Equal(t, "ministry:01", org1.ParentID)

	// Check parent-child
	assert.Len(t, index.ByParent["ministry:02"], 2)

	// Check ministry name from attr
	m1 := index.Entities["ministry:01"]
	require.NotNil(t, m1)
	assert.Equal(t, "Test Ministry One", m1.Name)
}

func TestParseXMLEntities_Empty(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<root></root>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)
	assert.Equal(t, 0, index.Stats.TotalEntities)
}

func TestParseXMLEntities_InvalidXML(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0"?><root><unclosed>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	assert.Error(t, err)
}

func TestParseXMLEntities_NameFromChildElement(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0"?>
<root>
  <item code="A1">
    <n>Item Name From N Element</n>
  </item>
</root>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	item := index.Entities["item:A1"]
	require.NotNil(t, item)
	assert.Equal(t, "Item Name From N Element", item.Name)
}

func TestParseXMLEntities_NameFromAttribute(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0"?>
<root>
  <item code="B1" name="Item Name From Attr"/>
</root>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	item := index.Entities["item:B1"]
	require.NotNil(t, item)
	assert.Equal(t, "Item Name From Attr", item.Name)
}

// --- NEW TESTS for child element text extraction ---

func TestParseXMLEntities_NameFromNameElement(t *testing.T) {
	// Tests that <name> child element works in addition to <n>
	xmlData := []byte(`<?xml version="1.0"?>
<root>
  <item code="C1">
    <name>Item Name From Name Element</name>
  </item>
</root>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	item := index.Entities["item:C1"]
	require.NotNil(t, item)
	assert.Equal(t, "Item Name From Name Element", item.Name)
}

func TestParseXMLEntities_ChildElementsAsAttributes(t *testing.T) {
	// Tests that child element text is stored in Attributes map
	xmlData := []byte(`<?xml version="1.0"?>
<root>
  <category code="P-1-1">
    <n>Test Category</n>
    <description>This is a test description with IETVER and NEIETVER content.</description>
    <departmentRef>LN</departmentRef>
  </category>
</root>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	cat := index.Entities["category:P-1-1"]
	require.NotNil(t, cat)
	assert.Equal(t, "Test Category", cat.Name)
	assert.Equal(t, "This is a test description with IETVER and NEIETVER content.", cat.Attributes["description"])
	assert.Equal(t, "LN", cat.Attributes["departmentRef"])
}

func TestParseXMLEntities_MultiValueChildElements(t *testing.T) {
	// Tests that multiple child elements with same name are concatenated
	xmlData := []byte(`<?xml version="1.0"?>
<root>
  <category code="P-1-1">
    <n>Multi Dept Category</n>
    <departmentRef>LN</departmentRef>
    <departmentRef>IPD</departmentRef>
    <departmentRef>DTD</departmentRef>
  </category>
</root>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	cat := index.Entities["category:P-1-1"]
	require.NotNil(t, cat)
	assert.Equal(t, "LN, IPD, DTD", cat.Attributes["departmentRef"])
}

func TestParseXMLEntities_NamespacedElements(t *testing.T) {
	// Go's encoding/xml strips namespace prefixes: vdvc:name → name
	xmlData := []byte(`<?xml version="1.0"?>
<vdvc:classification xmlns:vdvc="urn:vdvc:classification:2026" version="1.0.0">
  <vdvc:domain code="P">
    <vdvc:name>Pārvalde</vdvc:name>
    <vdvc:group code="P-1">
      <vdvc:name>Iestādes vadība</vdvc:name>
      <vdvc:category code="P-1-1">
        <vdvc:name>Test Category</vdvc:name>
        <vdvc:description>Test description with NEIETVER cross-references.</vdvc:description>
        <vdvc:departmentRef>LN</vdvc:departmentRef>
      </vdvc:category>
    </vdvc:group>
  </vdvc:domain>
</vdvc:classification>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	// Domain
	dom := index.Entities["domain:P"]
	require.NotNil(t, dom)
	assert.Equal(t, "Pārvalde", dom.Name)

	// Group
	grp := index.Entities["group:P-1"]
	require.NotNil(t, grp)
	assert.Equal(t, "Iestādes vadība", grp.Name)
	assert.Equal(t, "domain:P", grp.ParentID)

	// Category with description
	cat := index.Entities["category:P-1-1"]
	require.NotNil(t, cat)
	assert.Equal(t, "Test Category", cat.Name)
	assert.Equal(t, "Test description with NEIETVER cross-references.", cat.Attributes["description"])
	assert.Equal(t, "LN", cat.Attributes["departmentRef"])
	assert.Equal(t, "group:P-1", cat.ParentID)
}

func TestParseXMLEntities_SearchableDescriptions(t *testing.T) {
	// End-to-end: verify descriptions are searchable via SearchEntities
	xmlData := []byte(`<?xml version="1.0"?>
<root>
  <domain code="P">
    <n>Pārvalde</n>
    <category code="P-1-13">
      <n>Sarakste</n>
      <description>Sarakste ar ministrijām un valsts aģentūrām. NEIETVER: informācijas pieprasījumus pēc IAL (P-7-3).</description>
    </category>
    <category code="P-7-3">
      <n>Informācijas pieprasījumi</n>
      <description>Pieprasījumi saskaņā ar Informācijas atklātības likumu. NEIETVER: vispārējo saraksti (P-1-13).</description>
    </category>
  </domain>
</root>`)

	index := &EntityIndex{
		Entities: make(map[string]*Entity),
		ByType:   make(map[string][]string),
		ByParent: make(map[string][]string),
		Stats:    IndexStats{TypeCounts: make(map[string]int)},
	}

	err := parseXMLEntities(xmlData, index)
	require.NoError(t, err)

	// Search by description keyword — should find P-1-13
	results := index.SearchEntities("ministrijām", 10)
	require.Len(t, results, 1)
	assert.Equal(t, "category:P-1-13", results[0].ID)

	// Search by NEIETVER cross-reference
	results = index.SearchEntities("atklātības likum", 10)
	require.Len(t, results, 1)
	assert.Equal(t, "category:P-7-3", results[0].ID)

	// Search by name still works
	results = index.SearchEntities("sarakste", 10)
	assert.True(t, len(results) >= 1)
}
