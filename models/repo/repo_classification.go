// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

const (
	RepoClassificationTypeProcess   = "process"
	RepoClassificationTypeDecision  = "decision"
	RepoClassificationTypeReference = "reference"
	RepoClassificationTypeConnector = "connector"
	RepoClassificationDefaultType   = RepoClassificationTypeProcess

	RepoClassificationStatusDraft      = "draft"
	RepoClassificationStatusStable     = "stable"
	RepoClassificationStatusDeprecated = "deprecated"
	RepoClassificationStatusArchived   = "archived"
)

var (
	allowedRepoClassificationTypes = []string{
		RepoClassificationTypeProcess,
		RepoClassificationTypeDecision,
		RepoClassificationTypeReference,
		RepoClassificationTypeConnector,
	}
	allowedRepoClassificationStatuses = []string{
		RepoClassificationStatusDraft,
		RepoClassificationStatusStable,
		RepoClassificationStatusDeprecated,
		RepoClassificationStatusArchived,
	}
	allowedRepoReferenceKinds = []string{
		"schema",
		"classifier",
		"register",
		"codelist",
		"vocabulary",
		"standard",
	}
)

func init() {
	db.RegisterModel(new(RepoClassification))
}

// RepoClassification stores platform-level classification for repositories.
type RepoClassification struct {
	RepoID        int64              `xorm:"pk"`
	RepoType      string             `xorm:"VARCHAR(30) NOT NULL DEFAULT 'process'"`
	UAPFLevel     *int               `xorm:"null"`
	ReferenceKind string             `xorm:"VARCHAR(50)"`
	Status        string             `xorm:"VARCHAR(30) NOT NULL DEFAULT 'draft'"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	UpdatedBy     int64
}

func (RepoClassification) TableName() string {
	return "repo_classification"
}

// ValidateRepoType ensures the repo_type is allowed.
func ValidateRepoType(repoType string) error {
	if repoType == "" {
		return errors.New("repo_type is required")
	}
	if !slices.Contains(allowedRepoClassificationTypes, repoType) {
		return fmt.Errorf("invalid repo_type: %s", repoType)
	}
	return nil
}

// ValidateStatus ensures the status is allowed.
func ValidateStatus(status string) error {
	if status == "" {
		return errors.New("status is required")
	}
	if !slices.Contains(allowedRepoClassificationStatuses, status) {
		return fmt.Errorf("invalid status: %s", status)
	}
	return nil
}

// ValidateUAPFLevel validates the optional UAPF Level (0..4).
func ValidateUAPFLevel(level *int) error {
	if level == nil {
		return nil
	}
	if *level < 0 || *level > 4 {
		return fmt.Errorf("invalid uapf_level: %d (expected 0..4)", *level)
	}
	return nil
}

// ValidateReferenceKind validates reference_kind relative to repo_type.
func ValidateReferenceKind(kind, repoType string) error {
	if repoType != RepoClassificationTypeReference {
		if strings.TrimSpace(kind) != "" {
			return fmt.Errorf("reference_kind is only allowed when repo_type is %q", RepoClassificationTypeReference)
		}
		return nil
	}

	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return nil
	}
	if !slices.Contains(allowedRepoReferenceKinds, trimmed) {
		return fmt.Errorf("invalid reference_kind: %s", trimmed)
	}
	return nil
}

// GetRepoClassification fetches the classification for a repository. Returns nil when not found.
func GetRepoClassification(ctx context.Context, repoID int64) (*RepoClassification, error) {
	rc := new(RepoClassification)
	has, err := db.GetEngine(ctx).ID(repoID).Get(rc)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rc, nil
}

func validateRepoClassification(rc *RepoClassification) error {
	if err := ValidateRepoType(rc.RepoType); err != nil {
		return err
	}
	if err := ValidateStatus(rc.Status); err != nil {
		return err
	}
	if err := ValidateUAPFLevel(rc.UAPFLevel); err != nil {
		return err
	}
	if rc.RepoType == RepoClassificationTypeReference && rc.UAPFLevel != nil {
		return errors.New("uapf_level must be null for reference repositories")
	}
	if err := ValidateReferenceKind(rc.ReferenceKind, rc.RepoType); err != nil {
		return err
	}
	return nil
}

// UpsertRepoClassification inserts or updates a classification row.
func UpsertRepoClassification(ctx context.Context, rc *RepoClassification) error {
	if rc == nil {
		return errors.New("repo classification is required")
	}
	rc.RepoType = strings.TrimSpace(rc.RepoType)
	rc.Status = strings.TrimSpace(rc.Status)
	rc.ReferenceKind = strings.TrimSpace(rc.ReferenceKind)
	if err := validateRepoClassification(rc); err != nil {
		return err
	}

	now := timeutil.TimeStampNow()
	existing, err := GetRepoClassification(ctx, rc.RepoID)
	if err != nil {
		return err
	}
	if existing == nil {
		rc.CreatedUnix = now
		rc.UpdatedUnix = now
		return db.Insert(ctx, rc)
	}

	existing.RepoType = rc.RepoType
	existing.UAPFLevel = rc.UAPFLevel
	existing.ReferenceKind = strings.TrimSpace(rc.ReferenceKind)
	existing.Status = rc.Status
	existing.UpdatedUnix = now
	existing.UpdatedBy = rc.UpdatedBy
	_, err = db.GetEngine(ctx).ID(existing.RepoID).AllCols().Update(existing)
	return err
}

// EnsureRepoClassificationDefault creates a default classification if missing.
func EnsureRepoClassificationDefault(ctx context.Context, repoID, actorUserID int64) error {
	existing, err := GetRepoClassification(ctx, repoID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	return UpsertRepoClassification(ctx, &RepoClassification{
		RepoID:    repoID,
		RepoType:  RepoClassificationDefaultType,
		Status:    RepoClassificationStatusDraft,
		UpdatedBy: actorUserID,
	})
}

// DeleteRepoClassification removes metadata for a repository.
func DeleteRepoClassification(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).ID(repoID).Delete(&RepoClassification{})
	return err
}
