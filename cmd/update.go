// Copyright 2026 The ProcessGit Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v3"
)

// CmdUpdate manages ProcessGit self-updates against GitHub Releases.
//
// Subcommands:
//
//	check     — query GitHub for the latest release and report whether an
//	            update is available. Read-only, network only, no DB or config.
//
// Planned (see Slice 2B):
//
//	download  — fetch the release manifest + image/binary artifacts and
//	            verify signatures, into a staging directory.
//	apply     — atomically swap the running binary / pull the new image,
//	            run migrations, and re-exec or trigger a restart.
var CmdUpdate = &cli.Command{
	Name:        "update",
	Usage:       "Check for and (later) apply ProcessGit updates from GitHub Releases",
	Description: "Talks to GitHub Releases to discover newer ProcessGit versions. For Docker deployments the updater sidecar is the recommended path; for bare-metal binary installs this command will (in a future release) download and apply updates in place.",
	Commands: []*cli.Command{
		cmdUpdateCheck,
	},
}

var cmdUpdateCheck = &cli.Command{
	Name:        "check",
	Usage:       "Check whether a newer ProcessGit release is available",
	Description: "Queries the GitHub Releases API for the latest published release (skipping pre-releases by default) and compares it against the running version. Exits 0 if up-to-date or a newer version is available; exits non-zero only on transport/parse errors. Use --json for machine-readable output.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "repo",
			Usage:   "GitHub repo in `OWNER/NAME` form",
			Value:   "Algomation-AI/ProcessGit",
			Sources: cli.EnvVars("PROCESSGIT_UPDATE_REPO"),
		},
		&cli.StringFlag{
			Name:    "channel",
			Usage:   "Release channel: `stable` (skip pre-releases) or `prerelease` (include them)",
			Value:   "stable",
			Sources: cli.EnvVars("PROCESSGIT_UPDATE_CHANNEL"),
		},
		&cli.StringFlag{
			Name:    "github-api",
			Usage:   "GitHub API base URL (override for GitHub Enterprise)",
			Value:   "https://api.github.com",
			Sources: cli.EnvVars("PROCESSGIT_UPDATE_GITHUB_API"),
		},
		&cli.StringFlag{
			Name:    "github-token",
			Usage:   "Optional GitHub token (raises rate limit; no scopes needed for public repos)",
			Sources: cli.EnvVars("PROCESSGIT_UPDATE_GITHUB_TOKEN", "GITHUB_TOKEN"),
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "HTTP timeout for the GitHub API request",
			Value: 15 * time.Second,
		},
		&cli.BoolFlag{
			Name:    "json",
			Usage:   "Emit JSON instead of human-readable output",
			Sources: cli.EnvVars("PROCESSGIT_UPDATE_JSON"),
		},
	},
	Action: runUpdateCheck,
}

// updateCheckResult is the schema of --json output.
type updateCheckResult struct {
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	LatestTag       string `json:"latest_tag,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	Prerelease      bool   `json:"prerelease"`
	ReleaseURL      string `json:"release_url,omitempty"`
	ReleasedAt      string `json:"released_at,omitempty"`
	ReleaseName     string `json:"release_name,omitempty"`
	Channel         string `json:"channel"`
	Repo            string `json:"repo"`
	Note            string `json:"note,omitempty"`
}

// ghRelease is the subset of the GitHub Releases API response we care about.
type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

func runUpdateCheck(ctx context.Context, c *cli.Command) error {
	repo := c.String("repo")
	channel := strings.ToLower(c.String("channel"))
	apiBase := strings.TrimRight(c.String("github-api"), "/")
	token := c.String("github-token")
	timeout := c.Duration("timeout")
	jsonOut := c.Bool("json")

	switch channel {
	case "stable", "prerelease":
	default:
		return fmt.Errorf("invalid --channel %q (expected `stable` or `prerelease`)", channel)
	}
	if !strings.Contains(repo, "/") {
		return fmt.Errorf("invalid --repo %q (expected `OWNER/NAME`)", repo)
	}

	rel, err := fetchLatestRelease(ctx, apiBase, repo, channel, token, timeout)
	if err != nil {
		return fmt.Errorf("query GitHub Releases: %w", err)
	}

	current := strings.TrimSpace(setting.AppVer)
	res := updateCheckResult{
		Current: current,
		Channel: channel,
		Repo:    repo,
	}
	if rel == nil {
		res.Note = "no releases found"
		return emit(res, jsonOut)
	}

	res.LatestTag = rel.TagName
	res.Latest = strings.TrimPrefix(rel.TagName, "v")
	res.Prerelease = rel.Prerelease
	res.ReleaseURL = rel.HTMLURL
	res.ReleaseName = rel.Name
	if !rel.PublishedAt.IsZero() {
		res.ReleasedAt = rel.PublishedAt.UTC().Format(time.RFC3339)
	}

	cmpResult, cmpOK := semverCompare(current, res.Latest)
	switch {
	case !cmpOK:
		res.Note = "could not compare versions; current build does not look like semver — assume update available"
		res.UpdateAvailable = true
	case cmpResult < 0:
		res.UpdateAvailable = true
	default:
		res.UpdateAvailable = false
	}

	return emit(res, jsonOut)
}

func emit(r updateCheckResult, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}

	w := os.Stdout
	fmt.Fprintf(w, "Repository:      %s\n", r.Repo)
	fmt.Fprintf(w, "Channel:         %s\n", r.Channel)
	fmt.Fprintf(w, "Current version: %s\n", orDash(r.Current))
	fmt.Fprintf(w, "Latest version:  %s", orDash(r.Latest))
	if r.LatestTag != "" && r.LatestTag != "v"+r.Latest {
		fmt.Fprintf(w, " (tag %s)", r.LatestTag)
	}
	if r.Prerelease {
		fmt.Fprint(w, " [pre-release]")
	}
	fmt.Fprintln(w)
	if r.ReleasedAt != "" {
		fmt.Fprintf(w, "Released at:     %s\n", r.ReleasedAt)
	}
	if r.ReleaseURL != "" {
		fmt.Fprintf(w, "Release notes:   %s\n", r.ReleaseURL)
	}
	fmt.Fprintln(w)
	switch {
	case r.Latest == "":
		fmt.Fprintf(w, "Status:          no published release found\n")
	case r.UpdateAvailable:
		fmt.Fprintf(w, "Status:          update available (%s → %s)\n", orDash(r.Current), r.Latest)
	default:
		fmt.Fprintf(w, "Status:          up to date\n")
	}
	if r.Note != "" {
		fmt.Fprintf(w, "Note:            %s\n", r.Note)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// fetchLatestRelease returns the most recent release matching the channel.
// For `stable`, it walks `/releases` and returns the newest non-draft,
// non-prerelease entry (the `/releases/latest` endpoint also does this but
// returns 404 if no stable releases exist; walking is more robust for
// brand-new repos that have only pre-releases).
// For `prerelease`, it returns the newest non-draft entry of any kind.
func fetchLatestRelease(ctx context.Context, apiBase, repo, channel, token string, timeout time.Duration) (*ghRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=30", apiBase, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "processgit-update-check")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repo %q not found (or releases endpoint unavailable)", repo)
	}
	if resp.StatusCode == http.StatusForbidden {
		// Likely rate-limited
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github API 403: %s", strings.TrimSpace(string(body)))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var releases []ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	for i := range releases {
		r := releases[i]
		if r.Draft {
			continue
		}
		if channel == "stable" && r.Prerelease {
			continue
		}
		return &r, nil
	}
	return nil, nil
}

// semverCompare returns -1 / 0 / 1 if a < b / a == b / a > b, and a bool
// indicating whether both inputs were parseable as semver.
//
// Implements the relevant subset of semver 2.0.0: M.m.p[-pre][+build].
// Build metadata is ignored. Pre-release ordering follows the spec.
func semverCompare(a, b string) (int, bool) {
	pa, ok := parseSemver(a)
	if !ok {
		return 0, false
	}
	pb, ok := parseSemver(b)
	if !ok {
		return 0, false
	}
	if c := cmpInts(pa.major, pb.major); c != 0 {
		return c, true
	}
	if c := cmpInts(pa.minor, pb.minor); c != 0 {
		return c, true
	}
	if c := cmpInts(pa.patch, pb.patch); c != 0 {
		return c, true
	}
	// Pre-release: a version WITH pre-release has lower precedence than the same
	// version WITHOUT pre-release.
	switch {
	case len(pa.pre) == 0 && len(pb.pre) == 0:
		return 0, true
	case len(pa.pre) == 0:
		return 1, true
	case len(pb.pre) == 0:
		return -1, true
	}
	return cmpPre(pa.pre, pb.pre), true
}

type parsedSemver struct {
	major, minor, patch int
	pre                 []string
}

func parseSemver(s string) (parsedSemver, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return parsedSemver{}, false
	}
	// Strip build metadata.
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return parsedSemver{}, false
	}
	p := parsedSemver{}
	for i, q := range []*int{&p.major, &p.minor, &p.patch} {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return parsedSemver{}, false
		}
		*q = n
	}
	if pre != "" {
		p.pre = strings.Split(pre, ".")
	}
	return p, true
}

func cmpInts(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpPre(a, b []string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ai, aIsNum := strconv.Atoi(a[i])
		bi, bIsNum := strconv.Atoi(b[i])
		anum := aIsNum == nil
		bnum := bIsNum == nil
		switch {
		case anum && bnum:
			if c := cmpInts(ai, bi); c != 0 {
				return c
			}
		case anum && !bnum:
			return -1 // numeric < alphanumeric
		case !anum && bnum:
			return 1
		default:
			if a[i] < b[i] {
				return -1
			}
			if a[i] > b[i] {
				return 1
			}
		}
	}
	// Longer pre-release identifier list wins ties at the prefix
	return cmpInts(len(a), len(b))
}

// Sentinel for callers that might want to programmatically detect "no releases".
var errNoReleases = errors.New("no releases")

var _ = errNoReleases
