// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

const (
	defaultRepoClassificationType   = "process"
	defaultRepoClassificationStatus = "draft"
)

// RepoClassificationDefault aligns the repo_classification table with default values.
type RepoClassificationDefault struct {
	RepoID        int64              `xorm:"pk"`
	RepoType      string             `xorm:"VARCHAR(30) NOT NULL DEFAULT 'process' INDEX idx_repo_classification_type"`
	UAPFLevel     *int               `xorm:"null INDEX idx_repo_classification_level"`
	ReferenceKind string             `xorm:"VARCHAR(50)"`
	Status        string             `xorm:"VARCHAR(30) NOT NULL DEFAULT 'draft' INDEX idx_repo_classification_status"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	UpdatedBy     int64              `xorm:"index"`
}

func (RepoClassificationDefault) TableName() string { return "repo_classification" }

type repoIDOnly struct {
	ID int64 `xorm:"pk autoincr"`
}

func (repoIDOnly) TableName() string { return "repository" }

// SetRepoClassificationDefault adds a default classification value and backfills missing rows.
func SetRepoClassificationDefault(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := base.RecreateTable(sess, new(RepoClassificationDefault)); err != nil {
		_ = sess.Rollback()
		return err
	}

	var repos []repoIDOnly
	if err := sess.Table("repository").Select("id").Find(&repos); err != nil {
		_ = sess.Rollback()
		return err
	}

	now := timeutil.TimeStampNow()
	for _, repo := range repos {
		exists, err := sess.Table("repo_classification").Where("repo_id = ?", repo.ID).
			Exist(new(RepoClassificationDefault))
		if err != nil {
			_ = sess.Rollback()
			return err
		}
		if exists {
			continue
		}

		rc := &RepoClassificationDefault{
			RepoID:      repo.ID,
			RepoType:    defaultRepoClassificationType,
			Status:      defaultRepoClassificationStatus,
			CreatedUnix: now,
			UpdatedUnix: now,
		}

		if _, err := sess.Insert(rc); err != nil {
			_ = sess.Rollback()
			return err
		}
	}

	return sess.Commit()
}
