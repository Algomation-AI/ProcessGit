// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"
)

func TestFeedRepo(t *testing.T) {
	t.Run("AtomDisabled", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		// Feed support has been removed; .atom URLs should return 404
		req := NewRequest(t, "GET", "/user2/repo1.atom")
		MakeRequest(t, req, http.StatusNotFound)
	})
}
