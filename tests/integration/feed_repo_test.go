// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestFeedRepo(t *testing.T) {
	t.Run("Atom", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		req := NewRequest(t, "GET", "/user2/repo1.atom")
		resp := MakeRequest(t, req, http.StatusOK)

		data := resp.Body.String()
		assert.Contains(t, data, `<feed xmlns="http://www.w3.org/2005/Atom"`)
	})
}
