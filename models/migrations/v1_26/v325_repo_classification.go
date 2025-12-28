// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// RepoClassification defines the classification metadata stored for repositories.
type RepoClassification struct {
	RepoID        int64              `xorm:"pk"`
	RepoType      string             `xorm:"VARCHAR(30) NOT NULL INDEX idx_repo_classification_type"`
	UAPFLevel     *int               `xorm:"null INDEX idx_repo_classification_level"`
	ReferenceKind string             `xorm:"VARCHAR(50)"`
	Status        string             `xorm:"VARCHAR(30) NOT NULL DEFAULT 'draft' INDEX idx_repo_classification_status"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	UpdatedBy     int64              `xorm:"index"`
}

func (RepoClassification) TableName() string {
	return "repo_classification"
}

// AddRepoClassificationTable creates the repo_classification table for repository metadata.
func AddRepoClassificationTable(x *xorm.Engine) error {
	return x.Sync(new(RepoClassification))
}
