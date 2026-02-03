// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package processgitviewer

import "path"

func joinFromDir(dir, name string) string {
	if dir == "" {
		return name
	}
	return path.Join(dir, name)
}
