// Copyright 2026 The ProcessGit Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Admin updates page — surfaces the processgit-updater sidecar's status
// in the admin UI so operators can check for, trigger, and observe
// updates without curl-ing the sidecar's internal API by hand.
//
// All HTTP calls to the sidecar happen here, server-side. The browser
// never talks to the sidecar directly (it's on the compose-internal
// network and not exposed to the host). The bearer token never leaves
// the main app container.

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	gitea_context "code.gitea.io/gitea/services/context"
)

const (
	tplUpdates   templates.TplName = "admin/updates"
	tplUpdateJob templates.TplName = "admin/update_job"
)

// --- sidecar client --------------------------------------------------------

// updaterClient is a tiny HTTP client for talking to the
// processgit-updater sidecar. It reads its config from env vars set on
// the main container by the compose file:
//
//	PROCESSGIT_UPDATER_URL    e.g. http://processgit-updater:9000
//	PROCESSGIT_UPDATER_TOKEN  shared bearer
//
// If either is unset, NewUpdaterClient returns nil and the handler
// renders the "disabled" state.
type updaterClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newUpdaterClient() *updaterClient {
	url := strings.TrimRight(os.Getenv("PROCESSGIT_UPDATER_URL"), "/")
	tok := os.Getenv("PROCESSGIT_UPDATER_TOKEN")
	if url == "" || tok == "" {
		return nil
	}
	return &updaterClient{
		baseURL: url,
		token:   tok,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *updaterClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("updater request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("updater HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	}
	return nil
}

// --- value types mirrored from the updater's API ---------------------------

// UpdaterStatus is the shape of GET /status from the sidecar.
type UpdaterStatus struct {
	Version         string      `json:"version"`
	ActiveJob       *UpdaterJob `json:"active_job"`
	RecentJobsCount int         `json:"recent_jobs_count"`
}

// UpdaterRelease is the shape of GET /releases/latest.
type UpdaterRelease struct {
	Tag         string    `json:"tag"`
	Name        string    `json:"name"`
	Prerelease  bool      `json:"prerelease"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// UpdaterStep is one phase of an UpdaterJob.
type UpdaterStep struct {
	State       string     `json:"state"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// UpdaterJob is the shape of an update job (POST /update return + GET /update/{id}).
type UpdaterJob struct {
	ID            string        `json:"id"`
	State         string        `json:"state"`
	TargetTag     string        `json:"target_tag"`
	TargetVersion string        `json:"target_version,omitempty"`
	TargetImage   string        `json:"target_image,omitempty"`
	TargetDigest  string        `json:"target_digest,omitempty"`
	PreviousImage string        `json:"previous_image,omitempty"`
	StartedAt     time.Time     `json:"started_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
	Steps         []UpdaterStep `json:"steps"`
	Error         string        `json:"error,omitempty"`
}

// IsTerminal reports whether the job has reached a final state.
func (j *UpdaterJob) IsTerminal() bool {
	switch j.State {
	case "committed", "rolled_back", "failed", "aborted":
		return true
	}
	return false
}

// IsSuccess reports whether the job committed cleanly.
func (j *UpdaterJob) IsSuccess() bool { return j.State == "committed" }

// IsFailure reports whether the job ended in a failure state.
func (j *UpdaterJob) IsFailure() bool {
	return j.State == "failed" || j.State == "rolled_back" || j.State == "aborted"
}

type historyResponse struct {
	Jobs []*UpdaterJob `json:"jobs"`
}

type updateStartRequest struct {
	TargetTag string `json:"target_tag"`
}

type updateStartResponse struct {
	JobID     string      `json:"job_id"`
	StatusURL string      `json:"status_url"`
	Job       *UpdaterJob `json:"job"`
}

// --- handlers --------------------------------------------------------------

// Updates is the main /-/admin/updates page.
func Updates(ctx *gitea_context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.updates")
	ctx.Data["PageIsAdminUpdates"] = true
	ctx.Data["CurrentVersion"] = setting.AppVer

	c := newUpdaterClient()
	if c == nil {
		ctx.Data["UpdaterEnabled"] = false
		ctx.HTML(http.StatusOK, tplUpdates)
		return
	}
	ctx.Data["UpdaterEnabled"] = true

	// Fetch active/recent state.
	var status UpdaterStatus
	if err := c.do(ctx, http.MethodGet, "/status", nil, &status); err != nil {
		log.Warn("admin.Updates: /status: %v", err)
		ctx.Data["UpdaterError"] = err.Error()
		ctx.HTML(http.StatusOK, tplUpdates)
		return
	}
	ctx.Data["UpdaterStatus"] = status
	ctx.Data["ActiveJob"] = status.ActiveJob

	// Fetch latest release (best-effort — soft-fail).
	var latest UpdaterRelease
	if err := c.do(ctx, http.MethodGet, "/releases/latest", nil, &latest); err != nil {
		log.Warn("admin.Updates: /releases/latest: %v", err)
		ctx.Data["LatestReleaseError"] = err.Error()
	} else {
		ctx.Data["LatestRelease"] = latest
		ctx.Data["UpdateAvailable"] = isNewerVersion(latest.Tag, setting.AppVer)
	}

	// Fetch history (best-effort).
	var hist historyResponse
	if err := c.do(ctx, http.MethodGet, "/history", nil, &hist); err != nil {
		log.Warn("admin.Updates: /history: %v", err)
	} else {
		ctx.Data["UpdaterHistory"] = hist.Jobs
	}

	ctx.HTML(http.StatusOK, tplUpdates)
}

// UpdatesInstallPost is the POST /-/admin/updates/install handler.
// Triggers an update against the given target_tag and redirects to the
// job detail page.
func UpdatesInstallPost(ctx *gitea_context.Context) {
	targetTag := strings.TrimSpace(ctx.FormString("target_tag"))
	if targetTag == "" {
		ctx.Flash.Error(ctx.Tr("admin.updates.error_no_tag"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/updates")
		return
	}

	c := newUpdaterClient()
	if c == nil {
		ctx.Flash.Error(ctx.Tr("admin.updates.error_disabled"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/updates")
		return
	}

	var resp updateStartResponse
	if err := c.do(ctx, http.MethodPost, "/update", updateStartRequest{TargetTag: targetTag}, &resp); err != nil {
		ctx.Flash.Error(fmt.Sprintf("%s: %s", ctx.Tr("admin.updates.install_failed"), err.Error()))
		ctx.Redirect(setting.AppSubURL + "/-/admin/updates")
		return
	}

	ctx.Flash.Success(ctx.Tr("admin.updates.install_started", targetTag))
	if resp.JobID == "" {
		ctx.Redirect(setting.AppSubURL + "/-/admin/updates")
		return
	}
	ctx.Redirect(setting.AppSubURL + "/-/admin/updates/jobs/" + resp.JobID)
}

// UpdateJobView is the GET /-/admin/updates/jobs/{jobid} handler.
func UpdateJobView(ctx *gitea_context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.updates.job_title")
	ctx.Data["PageIsAdminUpdates"] = true

	jobID := ctx.PathParam("jobid")
	if jobID == "" {
		ctx.NotFound("UpdateJobView", errors.New("missing job id"))
		return
	}

	c := newUpdaterClient()
	if c == nil {
		ctx.NotFound("UpdateJobView", errors.New("updater not configured"))
		return
	}

	var job UpdaterJob
	if err := c.do(ctx, http.MethodGet, "/update/"+jobID, nil, &job); err != nil {
		// 404 from updater also lands here; surface as not-found rather than 500.
		ctx.NotFound("UpdateJobView", err)
		return
	}
	ctx.Data["Job"] = &job

	// Trigger meta-refresh on the page if the job is still running so the
	// browser polls without JS. 2-second cadence matches the sidecar's
	// internal state-machine step granularity.
	if !job.IsTerminal() {
		ctx.Data["AutoRefreshSeconds"] = 2
	}

	ctx.HTML(http.StatusOK, tplUpdateJob)
}

// --- helpers ---------------------------------------------------------------

// isNewerVersion is a deliberately-conservative semver compare for the
// "Update available" banner. It strips a leading "v" from both sides,
// then does a tuple-wise compare of the dot-separated parts. Returns
// false on parse failure, on equal versions, or when latest <= current.
//
// We don't pull in a semver library because the updater's manifest
// already enforces strict semver on releases — anything we'd parse
// here is already validated upstream.
func isNewerVersion(latest, current string) bool {
	if latest == "" || current == "" {
		return false
	}
	l := strings.Split(strings.TrimPrefix(strings.TrimPrefix(latest, "v"), "V"), ".")
	c := strings.Split(strings.TrimPrefix(strings.TrimPrefix(current, "v"), "V"), ".")
	// Strip any "+suffix" build metadata or "-rc1" pre-release tag from the
	// last component, comparing only the numeric core. Pre-releases sort
	// older than their corresponding release (so 0.1.0-rc1 < 0.1.0, but
	// our coarse check just compares integers and is correct for that case).
	for i, s := range l {
		l[i] = stripSuffix(s)
	}
	for i, s := range c {
		c[i] = stripSuffix(s)
	}
	n := len(l)
	if len(c) > n {
		n = len(c)
	}
	for i := 0; i < n; i++ {
		var li, ci int
		fmt.Sscanf(elemOr(l, i, "0"), "%d", &li)
		fmt.Sscanf(elemOr(c, i, "0"), "%d", &ci)
		if li > ci {
			return true
		}
		if li < ci {
			return false
		}
	}
	return false
}

func stripSuffix(s string) string {
	for _, sep := range []string{"-", "+"} {
		if i := strings.Index(s, sep); i >= 0 {
			return s[:i]
		}
	}
	return s
}

func elemOr(s []string, i int, def string) string {
	if i < len(s) {
		return s[i]
	}
	return def
}
