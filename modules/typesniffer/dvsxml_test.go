// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package typesniffer

import (
	"strings"
	"testing"
)

func TestDetectDVSXMLType(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantType   string
		wantNS     string
		wantSchema string
		wantOK     bool
	}{
		{
			name: "classification scheme",
			data: `<?xml version="1.0"?>
<KlasifikacijasShema xmlns="https://vdvc.gov.lv/schema/dvs/classification-scheme/v1" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="https://vdvc.gov.lv/schema/dvs/classification-scheme/v1 schema.xsd">
</KlasifikacijasShema>`,
			wantType:   "dvs.classification-scheme",
			wantNS:     "https://vdvc.gov.lv/schema/dvs/classification-scheme/v1",
			wantSchema: "https://vdvc.gov.lv/schema/dvs/classification-scheme/v1 schema.xsd",
			wantOK:     true,
		},
		{
			name:     "document metadata",
			data:     `<DvsDokumenti xmlns="https://vdvc.gov.lv/schema/dvs/document-metadata/v1"></DvsDokumenti>`,
			wantType: "dvs.document-metadata",
			wantNS:   "https://vdvc.gov.lv/schema/dvs/document-metadata/v1",
			wantOK:   true,
		},
		{
			name:     "unknown xml",
			data:     `<root xmlns="https://example.com/schema/v1"></root>`,
			wantType: "",
			wantOK:   false,
			wantNS:   "https://example.com/schema/v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			typ, meta, ok := DetectDVSXMLType([]byte(tc.data))
			if ok != tc.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tc.wantOK)
			}
			if typ != tc.wantType {
				t.Fatalf("type=%q, want %q", typ, tc.wantType)
			}
			if meta == nil {
				t.Fatalf("meta should not be nil")
			}
			if ns := meta["namespace"]; ns != tc.wantNS {
				t.Fatalf("namespace=%q, want %q", ns, tc.wantNS)
			}
			if tc.wantSchema != "" && meta["schemaLocation"] != tc.wantSchema {
				t.Fatalf("schemaLocation=%q, want %q", meta["schemaLocation"], tc.wantSchema)
			}
			if name := strings.TrimSpace(meta["localName"]); name == "" {
				t.Fatalf("localName should be captured")
			}
		})
	}
}
