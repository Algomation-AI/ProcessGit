// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

type processGitViewerPayload struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	RepoLink   string `json:"repoLink"`
	Branch     string `json:"branch"`
	Ref        string `json:"ref"`
	Path       string `json:"path"`
	Dir        string `json:"dir"`
	LastCommit string `json:"lastCommit"`

	EntryRawURL string            `json:"entryRawUrl"`
	Targets     map[string]string `json:"targets"`
	EditAllow   []string          `json:"editAllow"`
	APIURL      string            `json:"apiUrl"`
}
