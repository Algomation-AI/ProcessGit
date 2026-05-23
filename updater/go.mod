// processgit-updater is the sidecar that pulls and applies ProcessGit
// updates inside a Docker deployment. It's a separate Go module from the
// main ProcessGit codebase so that its dependency surface stays small and
// auditable — the sidecar has access to /var/run/docker.sock and runs
// privileged-by-implication, so every additional dependency is a risk.
module github.com/Algomation-AI/ProcessGit/updater

go 1.25
