// release-helper emits the per-release manifest (release.json) consumed by
// `processgit update check`, the updater sidecar, and external tooling.
//
// It reads everything from environment variables so it can be driven cleanly
// from a GitHub Actions workflow step without needing to pass complex CLI flags.
//
// Required env:
//
//	RELEASE_VERSION       e.g. "0.1.0"          (no leading v)
//	RELEASE_TAG           e.g. "v0.1.0"
//	RELEASE_PRERELEASE    "true" | "false"
//	IMAGE_REGISTRY        e.g. "ghcr.io"
//	IMAGE_REPOSITORY      e.g. "algomation-ai/processgit"
//	IMAGE_DIGEST          e.g. "sha256:abcd..."
//	IMAGE_PLATFORMS       comma-sep, e.g. "linux/amd64,linux/arm64"
//	IMAGE_ADDITIONAL_TAGS comma-sep, e.g. "0.1,0,latest" (optional)
//	SIGNING_ISSUER        e.g. "https://token.actions.githubusercontent.com"
//	SIGNING_IDENTITY_REGEX e.g. "^https://github.com/Algomation-AI/ProcessGit/\\.github/workflows/release\\.yml@.*"
//	RELEASE_NOTES_URL     e.g. "https://github.com/Algomation-AI/ProcessGit/releases/tag/v0.1.0"
//	BUILD_COMMIT          git sha
//	BUILD_WORKFLOW_RUN_URL workflow run URL (optional)
//	SOURCE_TARBALL_URL    URL to source tarball (optional)
//	SOURCE_TARBALL_SHA256 sha256 of source tarball (optional)
//	SOURCE_TARBALL_SIZE   byte size of source tarball (optional)
//	MIGRATION_REQUIRED    "true" | "false" (default false)
//	MIGRATION_COMMAND     e.g. "/app/gitea/gitea migrate" (optional)
//	OUTPUT                output path (defaults to stdout)
//
// Output is pretty-printed JSON conforming to build/release.schema.json.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type image struct {
	Registry       string   `json:"registry"`
	Repository     string   `json:"repository"`
	Tag            string   `json:"tag"`
	Digest         string   `json:"digest"`
	Platforms      []string `json:"platforms"`
	AdditionalTags []string `json:"additional_tags,omitempty"`
}

type source struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size,omitempty"`
}

type signing struct {
	Method        string `json:"method"`
	Issuer        string `json:"issuer"`
	IdentityRegex string `json:"identity_regex"`
	RekorLogIndex *int64 `json:"rekor_log_index,omitempty"`
}

type migration struct {
	Required                 bool   `json:"required"`
	Command                  string `json:"command,omitempty"`
	EstimatedDowntimeSeconds int    `json:"estimated_downtime_seconds,omitempty"`
}

type build struct {
	Commit         string `json:"commit,omitempty"`
	WorkflowRunURL string `json:"workflow_run_url,omitempty"`
	Builder        string `json:"builder,omitempty"`
}

type manifest struct {
	SchemaVersion        int       `json:"schema_version"`
	Name                 string    `json:"name"`
	Version              string    `json:"version"`
	Tag                  string    `json:"tag"`
	ReleasedAt           string    `json:"released_at"`
	Prerelease           bool      `json:"prerelease"`
	MinUpgradeFrom       *string   `json:"min_upgrade_from"`
	Image                image     `json:"image"`
	Binaries             []any     `json:"binaries"`
	Source               *source   `json:"source,omitempty"`
	Signing              signing   `json:"signing"`
	ReleaseNotesURL      string    `json:"release_notes_url"`
	ReleaseNotesMarkdown string    `json:"release_notes_markdown,omitempty"`
	Migration            migration `json:"migration"`
	BreakingChanges      []string  `json:"breaking_changes"`
	Deprecations         []string  `json:"deprecations"`
	Build                build     `json:"build"`
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fmt.Fprintf(os.Stderr, "release-helper: required env %s is empty\n", k)
		os.Exit(1)
	}
	return v
}

func envBool(k string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "release-helper: env %s=%q is not a bool: %v\n", k, v, err)
		os.Exit(1)
	}
	return b
}

func envInt64(k string) int64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "release-helper: env %s=%q is not an int: %v\n", k, v, err)
		os.Exit(1)
	}
	return n
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func main() {
	m := manifest{
		SchemaVersion:  1,
		Name:           "processgit",
		Version:        mustEnv("RELEASE_VERSION"),
		Tag:            mustEnv("RELEASE_TAG"),
		ReleasedAt:     time.Now().UTC().Format(time.RFC3339),
		Prerelease:     envBool("RELEASE_PRERELEASE", false),
		MinUpgradeFrom: nil,
		Image: image{
			Registry:       mustEnv("IMAGE_REGISTRY"),
			Repository:     mustEnv("IMAGE_REPOSITORY"),
			Tag:            mustEnv("RELEASE_VERSION"),
			Digest:         mustEnv("IMAGE_DIGEST"),
			Platforms:      splitCSV(mustEnv("IMAGE_PLATFORMS")),
			AdditionalTags: splitCSV(os.Getenv("IMAGE_ADDITIONAL_TAGS")),
		},
		Binaries: []any{}, // v0.1.0: no standalone binaries; container only
		Signing: signing{
			Method:        "cosign-keyless",
			Issuer:        mustEnv("SIGNING_ISSUER"),
			IdentityRegex: mustEnv("SIGNING_IDENTITY_REGEX"),
		},
		ReleaseNotesURL: mustEnv("RELEASE_NOTES_URL"),
		Migration: migration{
			Required: envBool("MIGRATION_REQUIRED", false),
			Command:  os.Getenv("MIGRATION_COMMAND"),
		},
		BreakingChanges: []string{},
		Deprecations:    []string{},
		Build: build{
			Commit:         os.Getenv("BUILD_COMMIT"),
			WorkflowRunURL: os.Getenv("BUILD_WORKFLOW_RUN_URL"),
			Builder:        "github-actions",
		},
	}

	if u := os.Getenv("SOURCE_TARBALL_URL"); u != "" {
		m.Source = &source{
			URL:    u,
			SHA256: mustEnv("SOURCE_TARBALL_SHA256"),
			Size:   envInt64("SOURCE_TARBALL_SIZE"),
		}
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "release-helper: marshal: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')

	target := os.Getenv("OUTPUT")
	if target == "" || target == "-" {
		_, _ = os.Stdout.Write(out)
		return
	}
	if err := os.WriteFile(target, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "release-helper: write %s: %v\n", target, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "release-helper: wrote %s (%d bytes)\n", target, len(out))
}
