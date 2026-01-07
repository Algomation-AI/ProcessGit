// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestEnsureRepoClassificationDefault(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(1)
	assert.NoError(t, repo_model.DeleteRepoClassification(t.Context(), repoID))

	assert.NoError(t, repo_model.EnsureRepoClassificationDefault(t.Context(), repoID, 2))

	rc, err := repo_model.GetRepoClassification(t.Context(), repoID)
	assert.NoError(t, err)
	if assert.NotNil(t, rc) {
		assert.Equal(t, repo_model.RepoClassificationTypeProcess, rc.RepoType)
		assert.Equal(t, repo_model.RepoClassificationStatusDraft, rc.Status)
		assert.Nil(t, rc.UAPFLevel)
		assert.EqualValues(t, 2, rc.UpdatedBy)
		assert.NotZero(t, rc.CreatedUnix)
		assert.NotZero(t, rc.UpdatedUnix)
	}
}

func TestUpsertRepoClassification(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(2)
	level := 2
	rc := &repo_model.RepoClassification{
		RepoID:    repoID,
		RepoType:  repo_model.RepoClassificationTypeDecision,
		Status:    repo_model.RepoClassificationStatusStable,
		UAPFLevel: &level,
		UpdatedBy: 3,
	}
	assert.NoError(t, repo_model.UpsertRepoClassification(t.Context(), rc))

	rcFetched, err := repo_model.GetRepoClassification(t.Context(), repoID)
	assert.NoError(t, err)
	if assert.NotNil(t, rcFetched) {
		assert.Equal(t, level, *rcFetched.UAPFLevel)
		assert.EqualValues(t, 3, rcFetched.UpdatedBy)
	}

	newLevel := 4
	rc.UAPFLevel = &newLevel
	rc.Status = repo_model.RepoClassificationStatusDeprecated
	rc.UpdatedBy = 5
	assert.NoError(t, repo_model.UpsertRepoClassification(t.Context(), rc))

	rcUpdated, err := repo_model.GetRepoClassification(t.Context(), repoID)
	assert.NoError(t, err)
	if assert.NotNil(t, rcUpdated) {
		assert.Equal(t, newLevel, *rcUpdated.UAPFLevel)
		assert.Equal(t, repo_model.RepoClassificationStatusDeprecated, rcUpdated.Status)
		assert.EqualValues(t, 5, rcUpdated.UpdatedBy)
	}
}

func TestRepoClassificationValidation(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	level := 1
	err := repo_model.UpsertRepoClassification(t.Context(), &repo_model.RepoClassification{
		RepoID:    3,
		RepoType:  repo_model.RepoClassificationTypeReference,
		Status:    repo_model.RepoClassificationStatusDraft,
		UAPFLevel: &level,
	})
	assert.Error(t, err)

	err = repo_model.UpsertRepoClassification(t.Context(), &repo_model.RepoClassification{
		RepoID:        3,
		RepoType:      repo_model.RepoClassificationTypeProcess,
		Status:        repo_model.RepoClassificationStatusDraft,
		ReferenceKind: "schema",
	})
	assert.Error(t, err)

	bad := -1
	err = repo_model.UpsertRepoClassification(t.Context(), &repo_model.RepoClassification{
		RepoID:    3,
		RepoType:  repo_model.RepoClassificationTypeProcess,
		Status:    repo_model.RepoClassificationStatusDraft,
		UAPFLevel: &bad,
	})
	assert.Error(t, err)
}

func TestDeleteRepoClassification(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(4)
	assert.NoError(t, repo_model.EnsureRepoClassificationDefault(t.Context(), repoID, 1))

	rc, err := repo_model.GetRepoClassification(t.Context(), repoID)
	assert.NoError(t, err)
	assert.NotNil(t, rc)

	assert.NoError(t, repo_model.DeleteRepoClassification(t.Context(), repoID))
	rc, err = repo_model.GetRepoClassification(t.Context(), repoID)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrRepoClassificationNotExist(err))
	assert.Nil(t, rc)
}
