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
