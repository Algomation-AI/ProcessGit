// Types mirroring the build/release.schema.json contract, plus the helpers
// that fetch a release manifest from a GitHub Release.
//
// We re-declare the types here (rather than importing from the main
// codebase) so the updater module stays independent and the contract
// boundary is explicit. Drift between the two is detected by the
// `processgit update check` JSON-output integration test.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Manifest struct {
	SchemaVersion        int       `json:"schema_version"`
	Name                 string    `json:"name"`
	Version              string    `json:"version"`
	Tag                  string    `json:"tag"`
	ReleasedAt           string    `json:"released_at"`
	Prerelease           bool      `json:"prerelease"`
	MinUpgradeFrom       *string   `json:"min_upgrade_from"`
	Image                Image     `json:"image"`
	Binaries             []Binary  `json:"binaries"`
	Source               *Source   `json:"source,omitempty"`
	Signing              Signing   `json:"signing"`
	ReleaseNotesURL      string    `json:"release_notes_url"`
	ReleaseNotesMarkdown string    `json:"release_notes_markdown,omitempty"`
	Migration            Migration `json:"migration"`
	BreakingChanges      []string  `json:"breaking_changes"`
	Deprecations         []string  `json:"deprecations"`
	Build                Build     `json:"build"`
}

type Image struct {
	Registry       string   `json:"registry"`
	Repository     string   `json:"repository"`
	Tag            string   `json:"tag"`
	Digest         string   `json:"digest"`
	Platforms      []string `json:"platforms"`
	AdditionalTags []string `json:"additional_tags,omitempty"`
}

func (i Image) Ref() string {
	return fmt.Sprintf("%s/%s:%s", i.Registry, i.Repository, i.Tag)
}

func (i Image) DigestRef() string {
	return fmt.Sprintf("%s/%s@%s", i.Registry, i.Repository, i.Digest)
}

type Binary struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	URL     string `json:"url"`
	Size    int64  `json:"size,omitempty"`
	SHA256  string `json:"sha256"`
	Variant string `json:"variant,omitempty"`
}

type Source struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size,omitempty"`
}

type Signing struct {
	Method        string `json:"method"`
	Issuer        string `json:"issuer"`
	IdentityRegex string `json:"identity_regex"`
	RekorLogIndex *int64 `json:"rekor_log_index,omitempty"`
}

type Migration struct {
	Required                 bool   `json:"required"`
	Command                  string `json:"command,omitempty"`
	EstimatedDowntimeSeconds int    `json:"estimated_downtime_seconds,omitempty"`
}

type Build struct {
	Commit         string `json:"commit,omitempty"`
	WorkflowRunURL string `json:"workflow_run_url,omitempty"`
	Builder        string `json:"builder,omitempty"`
}

// ghAsset is what the GitHub Releases API tells us about each release asset.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// ghRelease is the subset of the GitHub Releases API we care about.
type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

// GitHubClient is a tiny GitHub Releases REST client.
type GitHubClient struct {
	APIBase string // e.g. "https://api.github.com" (override for GHE)
	Repo    string // "OWNER/NAME"
	Token   string // optional; empty for unauthenticated
	HTTP    *http.Client
}

func NewGitHubClient(apiBase, repo, token string) *GitHubClient {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &GitHubClient{
		APIBase: strings.TrimRight(apiBase, "/"),
		Repo:    repo,
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitHubClient) do(ctx context.Context, method, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "processgit-updater")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github API %s %s: HTTP %d: %s", method, urlStr, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// LatestRelease walks /releases and returns the first non-draft release that
// matches the channel filter ("stable" excludes pre-releases; "prerelease"
// includes them).
func (c *GitHubClient) LatestRelease(ctx context.Context, channel string) (*ghRelease, error) {
	u := fmt.Sprintf("%s/repos/%s/releases?per_page=30", c.APIBase, c.Repo)
	body, err := c.do(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	var releases []ghRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parse releases: %w", err)
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
	return nil, fmt.Errorf("no releases matching channel %q", channel)
}

// ReleaseByTag fetches a single release by tag name.
func (c *GitHubClient) ReleaseByTag(ctx context.Context, tag string) (*ghRelease, error) {
	u := fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.APIBase, c.Repo, url.PathEscape(tag))
	body, err := c.do(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	var r ghRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &r, nil
}

// FetchManifest downloads release.json, release.json.sig, and release.json.crt
// from the given release and returns the parsed manifest plus the raw bytes of
// each (so the caller can pass them to cosign verify-blob).
type FetchedManifest struct {
	Manifest  *Manifest
	JSONBytes []byte
	Sig       []byte
	Cert      []byte
}

func (c *GitHubClient) FetchManifest(ctx context.Context, rel *ghRelease) (*FetchedManifest, error) {
	want := map[string]*[]byte{
		"release.json":     nil,
		"release.json.sig": nil,
		"release.json.crt": nil,
	}
	fm := &FetchedManifest{}
	want["release.json"] = &fm.JSONBytes
	want["release.json.sig"] = &fm.Sig
	want["release.json.crt"] = &fm.Cert

	for _, a := range rel.Assets {
		ptr, ok := want[a.Name]
		if !ok {
			continue
		}
		data, err := c.downloadAsset(ctx, a.BrowserDownloadURL)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", a.Name, err)
		}
		*ptr = data
	}
	for name, ptr := range want {
		if *ptr == nil {
			return nil, fmt.Errorf("release %s is missing required asset %q", rel.TagName, name)
		}
	}

	var m Manifest
	if err := json.Unmarshal(fm.JSONBytes, &m); err != nil {
		return nil, fmt.Errorf("parse release.json: %w", err)
	}
	if m.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported release manifest schema_version=%d (this updater supports 1)", m.SchemaVersion)
	}
	if m.Name != "processgit" {
		return nil, fmt.Errorf("release manifest name=%q (expected %q)", m.Name, "processgit")
	}
	fm.Manifest = &m
	return fm, nil
}

func (c *GitHubClient) downloadAsset(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	// Release assets are served from a CDN that follows the API auth header
	// only if it's a Bearer token on api.github.com; here we just use the
	// public download URL with no auth. Public-repo releases are world-readable.
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "processgit-updater")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download %s: HTTP %d: %s", downloadURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}
