// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package typesniffer

import (
	"bytes"
	"encoding/xml"
	"io"
)

// DVSXMLSniffLimit caps how many bytes are inspected when trying to detect a typed DVS XML.
const DVSXMLSniffLimit = 32 * 1024

// DetectDVSXMLType tries to detect ProcessGit "typed XML" documents used by DVS registries.
// It only looks at the first start element and inspects:
//   - the root element local name
//   - the default namespace
//   - optional xsi:schemaLocation
func DetectDVSXMLType(contentPrefix []byte) (typ string, meta map[string]string, ok bool) {
	if len(contentPrefix) == 0 {
		return "", nil, false
	}

	if len(contentPrefix) > DVSXMLSniffLimit {
		contentPrefix = contentPrefix[:DVSXMLSniffLimit]
	}

	meta = make(map[string]string)

	decoder := xml.NewDecoder(bytes.NewReader(contentPrefix))
	decoder.Strict = false

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", meta, false
		}

		start, okTok := token.(xml.StartElement)
		if !okTok {
			continue
		}

		meta["localName"] = start.Name.Local
		if start.Name.Space != "" {
			meta["namespace"] = start.Name.Space
		}

		for _, attr := range start.Attr {
			switch {
			case attr.Name.Space == "" && attr.Name.Local == "xmlns":
				meta["namespace"] = attr.Value
			case attr.Name.Local == "schemaLocation":
				meta["schemaLocation"] = attr.Value
			}
		}

		ns := meta["namespace"]
		switch {
		case ns == "https://vdvc.gov.lv/schema/dvs/classification-scheme/v1" && start.Name.Local == "KlasifikacijasShema":
			return "dvs.classification-scheme", meta, true
		case ns == "https://vdvc.gov.lv/schema/dvs/document-metadata/v1" && start.Name.Local == "DvsDokumenti":
			return "dvs.document-metadata", meta, true
		default:
			return "", meta, false
		}
	}

	return "", meta, false
}
