// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	templateMarkerPath         = "/data/.processgit/templates_bootstrapped"
	templateRootPath           = "/opt/processgit/repo-templates"
	templateConfigPath         = "/opt/processgit/bootstrap/template-repos.json"
	templateCommitName         = "ProcessGit Templates"
	templateCommitEmail        = "templates@processgit.org"
	templateClassificationType = "template"
)

type templateRepoConfig struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

func main() {
	if err := run(); err != nil {
		log.Error("[seed] %v", err)
		log.GetManager().Close()
		os.Exit(1)
	}
	log.GetManager().Close()
}

func run() error {
	var args setting.ArgWorkPathAndCustomConf
	flag.StringVar(&args.WorkPath, "work-path", "", "Set ProcessGit's working path")
	flag.StringVar(&args.CustomPath, "custom-path", "", "Set custom path")
	flag.StringVar(&args.CustomConf, "config", "", "Set custom config file")
	flag.Parse()

	setting.InitWorkPathAndCommonConfig(os.Getenv, args)
	setting.MustInstalled()
	setting.LoadSettings()
	logSeedRuntime()

	ctx := context.Background()
	if err := db.InitEngine(ctx); err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	if err := models.Init(ctx); err != nil {
		return fmt.Errorf("init models: %w", err)
	}
	if err := git.InitSimple(); err != nil {
		return fmt.Errorf("init git: %w", err)
	}

	if _, err := os.Stat(templateMarkerPath); err == nil {
		seedLogf("Templates already bootstrapped; skipping")
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check marker: %w", err)
	}

	if err := ensureDirExists(templateRootPath, "template root"); err != nil {
		return err
	}
	if err := ensureFileExists(templateConfigPath, "template repo config"); err != nil {
		return err
	}

	repos, err := loadTemplateRepoConfig(templateConfigPath)
	if err != nil {
		return err
	}

	ownerName := envOrDefault("PROCESSGIT_TEMPLATES_OWNER", "processgit-templates")
	ownerEmail := envOrDefault("PROCESSGIT_TEMPLATES_EMAIL", "processgit-templates@example.invalid")
	ownerPassword := envOrDefault("PROCESSGIT_TEMPLATES_PASSWORD", "processgit-templates")

	owner, err := ensureTemplatesOwner(ctx, ownerName, ownerEmail, ownerPassword)
	if err != nil {
		return err
	}

	seedStrict, err := parseSeedStrict()
	if err != nil {
		return err
	}

	seedLogf("Bootstrapping %d template repos", len(repos))
	hadFailure := false
	for _, repoCfg := range repos {
		repoName := repoCfg.Name
		if repoName == "" {
			repoName = "<unknown>"
		}
		err := func() error {
			if repoCfg.Name == "" {
				return fmt.Errorf("template repo entry missing name")
			}
			if repoCfg.Path == "" {
				return fmt.Errorf("template repo entry %q missing path", repoCfg.Name)
			}
			sourceDir := filepath.Join(templateRootPath, repoCfg.Path)
			if err := ensureDirExists(sourceDir, fmt.Sprintf("template content for %s", repoCfg.Name)); err != nil {
				return err
			}

			repo, err := ensureTemplateRepo(ctx, owner, repoCfg)
			if err != nil {
				return err
			}

			if err := ensureTemplateClassification(ctx, repo, owner); err != nil {
				return err
			}

			if err := ensureRepoContent(ctx, owner, repo, sourceDir); err != nil {
				return err
			}
			seedLogf("Template imported OK: %s/%s", owner.Name, repo.Name)
			return nil
		}()
		if err != nil {
			if seedStrict {
				return err
			}
			hadFailure = true
			log.Error("[seed] Template import failed for %s: %v", repoName, err)
			continue
		}
	}

	if hadFailure {
		seedLogf("Template bootstrap completed with failures; marker not written")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(templateMarkerPath), 0o755); err != nil {
		return fmt.Errorf("create marker dir: %w", err)
	}
	if err := os.WriteFile(templateMarkerPath, []byte("ok"), 0o644); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}

	seedLogf("Template bootstrap completed")
	return nil
}

func ensureDirExists(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found at %s", label, path)
		}
		return fmt.Errorf("stat %s: %w", label, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory: %s", label, path)
	}
	return nil
}

func ensureFileExists(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s not found at %s", label, path)
		}
		return fmt.Errorf("stat %s: %w", label, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected file: %s", label, path)
	}
	return nil
}

func loadTemplateRepoConfig(path string) ([]templateRepoConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template repo config: %w", err)
	}
	var repos []templateRepoConfig
	if err := json.Unmarshal(data, &repos); err != nil {
		return nil, fmt.Errorf("parse template repo config: %w", err)
	}
	return repos, nil
}

func ensureTemplatesOwner(ctx context.Context, name, email, password string) (*user_model.User, error) {
	owner, err := user_model.GetUserByName(ctx, name)
	if err == nil {
		seedLogf("Templates owner '%s' already exists", name)
		return owner, nil
	}
	if !user_model.IsErrUserNotExist(err) {
		return nil, fmt.Errorf("lookup templates owner: %w", err)
	}

	seedLogf("Creating templates owner '%s'", name)
	owner = &user_model.User{
		Name:               name,
		Email:              email,
		Passwd:             password,
		MustChangePassword: false,
	}
	overwrite := &user_model.CreateUserOverwriteOptions{
		IsRestricted: optional.Some(false),
		IsActive:     optional.Some(true),
	}
	if err := user_model.CreateUser(ctx, owner, &user_model.Meta{}, overwrite); err != nil {
		return nil, fmt.Errorf("create templates owner: %w", err)
	}
	return owner, nil
}

func ensureTemplateRepo(ctx context.Context, owner *user_model.User, cfg templateRepoConfig) (*repo_model.Repository, error) {
	repo, err := repo_model.GetRepositoryByName(ctx, owner.ID, cfg.Name)
	if err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			return nil, fmt.Errorf("lookup repo %s: %w", cfg.Name, err)
		}
		seedLogf("Creating template repo %s/%s", owner.Name, cfg.Name)
		return repo_service.CreateRepositoryDirectly(ctx, owner, owner, repo_service.CreateRepoOptions{
			Name:               cfg.Name,
			Description:        cfg.Description,
			IsPrivate:          false,
			IsTemplate:         true,
			AutoInit:           false,
			DefaultBranch:      setting.Repository.DefaultBranch,
			ClassificationType: templateClassificationType,
		}, true)
	}

	updatedCols := make([]string, 0, 2)
	if repo.Description != cfg.Description {
		repo.Description = cfg.Description
		updatedCols = append(updatedCols, "description")
	}
	if !repo.IsTemplate {
		repo.IsTemplate = true
		updatedCols = append(updatedCols, "is_template")
	}
	if len(updatedCols) > 0 {
		seedLogf("Updating template repo metadata for %s/%s", owner.Name, cfg.Name)
		if err := repo_model.UpdateRepositoryColsWithAutoTime(ctx, repo, "processgit-seed", updatedCols...); err != nil {
			return nil, fmt.Errorf("update repo %s: %w", cfg.Name, err)
		}
	}

	return repo, nil
}

func ensureTemplateClassification(ctx context.Context, repo *repo_model.Repository, doer *user_model.User) error {
	if repo.ID == 0 {
		return fmt.Errorf("template repo %s/%s has no id", repo.OwnerName, repo.Name)
	}
	seedLogf("Ensuring classification for repo_id=%d %s/%s", repo.ID, repo.OwnerName, repo.Name)
	desiredType := templateClassificationType
	desiredStatus := repo_model.RepoClassificationStatusDraft

	rc, err := repo_model.GetRepoClassification(ctx, repo.ID)
	if err != nil {
		if repo_model.IsErrRepoClassificationNotExist(err) {
			rc = nil
		} else {
			return fmt.Errorf("lookup repo classification for %s/%s: %w", repo.OwnerName, repo.Name, err)
		}
	}

	if rc == nil {
		rc = &repo_model.RepoClassification{
			RepoID:                      repo.ID,
			RepoType:                    desiredType,
			Status:                      desiredStatus,
			IdxRepoClassificationType:   desiredType,
			IdxRepoClassificationStatus: desiredStatus,
			UpdatedBy:                   doer.ID,
		}
		if err := repo_model.UpsertRepoClassification(ctx, rc); err != nil {
			return fmt.Errorf("create repo classification for %s/%s: %w", repo.OwnerName, repo.Name, err)
		}
		return nil
	}

	rc.RepoType = desiredType
	rc.Status = desiredStatus
	rc.IdxRepoClassificationType = desiredType
	rc.IdxRepoClassificationStatus = desiredStatus
	rc.UpdatedBy = doer.ID
	if err := repo_model.UpsertRepoClassification(ctx, rc); err != nil {
		return fmt.Errorf("upsert repo classification for %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	return nil
}

func ensureRepoContent(ctx context.Context, owner *user_model.User, repo *repo_model.Repository, sourceDir string) error {
	repoExists, err := gitrepo.IsRepositoryExist(ctx, repo)
	if err != nil {
		return fmt.Errorf("check repo path for %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	if !repoExists {
		seedLogf("Initializing git repository for %s/%s", repo.OwnerName, repo.Name)
		if err := gitrepo.InitRepository(ctx, repo, repo.ObjectFormatName); err != nil {
			return fmt.Errorf("init git repo %s/%s: %w", repo.OwnerName, repo.Name, err)
		}
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return fmt.Errorf("open git repo %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	defer gitRepo.Close()

	isEmpty, err := gitRepo.IsEmpty()
	if err != nil {
		return fmt.Errorf("check empty repo %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	if !isEmpty {
		seedLogf("Repo %s/%s already has content; skipping import", repo.OwnerName, repo.Name)
		return nil
	}

	seedLogf("Importing template content into %s/%s", repo.OwnerName, repo.Name)
	defaultBranch := repo.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = setting.Repository.DefaultBranch
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	tmpDir, cleanup, err := setting.AppDataTempDir("git-repo-content").MkdirTempRandom("template-seed-" + repo.Name)
	if err != nil {
		return fmt.Errorf("create temp dir for %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	defer cleanup()

	workDir := filepath.Join(tmpDir, "repo")
	if err := gitrepo.CloneRepoToLocal(ctx, repo, workDir, git.CloneRepoOptions{}); err != nil {
		return fmt.Errorf("git clone %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	if err := ensureGitWorkTree(ctx, workDir); err != nil {
		return err
	}
	if err := ensureOriginRemote(ctx, workDir, repo.RepoPath()); err != nil {
		return err
	}
	if err := ensureOriginVisible(ctx, workDir); err != nil {
		return err
	}
	if err := copyTemplateDir(sourceDir, workDir); err != nil {
		return fmt.Errorf("copy template content for %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	if err := commitAndPushTemplate(ctx, workDir, repo, owner, defaultBranch); err != nil {
		return err
	}

	repo.IsEmpty = false
	repo.DefaultBranch = defaultBranch
	if err := repo_model.UpdateRepositoryColsWithAutoTime(ctx, repo, "processgit-seed", "is_empty", "default_branch"); err != nil {
		return fmt.Errorf("update repo state for %s/%s: %w", repo.OwnerName, repo.Name, err)
	}
	return nil
}

func copyTemplateDir(sourceDir, destDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destDir, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode.IsDir() {
			return os.MkdirAll(target, mode.Perm())
		}
		if mode&os.ModeSymlink != 0 {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return util.CopyFile(path, target)
	})
}

func commitAndPushTemplate(ctx context.Context, workDir string, repo *repo_model.Repository, owner *user_model.User, defaultBranch string) error {
	commitTime := time.Now().Format(time.RFC3339)
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+templateCommitName,
		"GIT_AUTHOR_EMAIL="+templateCommitEmail,
		"GIT_AUTHOR_DATE="+commitTime,
		"GIT_COMMITTER_NAME="+templateCommitName,
		"GIT_COMMITTER_EMAIL="+templateCommitEmail,
		"GIT_COMMITTER_DATE="+commitTime,
	)
	pushEnv := append(repo_module.InternalPushingEnvironment(owner, repo), env...)

	if stdout, stderr, err := gitcmd.NewCommand("config").
		AddDynamicArguments("user.name", templateCommitName).
		WithDir(workDir).
		RunStdString(ctx); err != nil {
		log.Error("[seed] git config user.name failed: stdout=%s stderr=%s", stdout, stderr)
		return fmt.Errorf("git config user.name: %w; stdout: %s; stderr: %s", err, stdout, stderr)
	}
	if stdout, stderr, err := gitcmd.NewCommand("config").
		AddDynamicArguments("user.email", templateCommitEmail).
		WithDir(workDir).
		RunStdString(ctx); err != nil {
		log.Error("[seed] git config user.email failed: stdout=%s stderr=%s", stdout, stderr)
		return fmt.Errorf("git config user.email: %w; stdout: %s; stderr: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments("--is-inside-work-tree").
		WithDir(workDir).
		WithEnv(env).
		RunStdString(ctx); err != nil || strings.TrimSpace(stdout) != "true" {
		entries, readErr := os.ReadDir(workDir)
		if readErr != nil {
			log.Error("[seed] failed to read workDir for debug: %v", readErr)
		}
		for _, entry := range entries {
			log.Error("[seed] workDir entry: %s", entry.Name())
		}
		if err != nil {
			log.Error("[seed] git rev-parse failed: stdout=%s stderr=%s", stdout, stderr)
			return fmt.Errorf("seed workDir is not a git work tree: %q err=%w; stdout: %s; stderr: %s", stdout, err, stdout, stderr)
		}
		return fmt.Errorf("seed workDir is not a git work tree: %q; stdout: %s; stderr: %s", stdout, stdout, stderr)
	}

	if stdout, stderr, err := gitcmd.NewCommand("add").AddDynamicArguments("--all").WithDir(workDir).WithEnv(env).RunStdString(ctx); err != nil {
		log.Error("[seed] git add failed: stdout=%s stderr=%s", stdout, stderr)
		return fmt.Errorf("git add: %w; stdout: %s; stderr: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := gitcmd.NewCommand("commit").AddDynamicArguments("--message=Initial template import", "--no-gpg-sign").
		WithDir(workDir).
		WithEnv(env).
		RunStdString(ctx); err != nil {
		log.Error("[seed] git commit failed: stdout=%s stderr=%s", stdout, stderr)
		return fmt.Errorf("git commit: %w; stdout: %s; stderr: %s", err, stdout, stderr)
	}

	refspec := fmt.Sprintf("HEAD:refs/heads/%s", defaultBranch)
	if stdout, stderr, err := gitcmd.NewCommand("push").
		AddDynamicArguments("origin", refspec).
		WithDir(workDir).
		WithEnv(pushEnv).
		RunStdString(ctx); err != nil {
		log.Error("[seed] git push failed: stdout=%s stderr=%s", stdout, stderr)
		return fmt.Errorf("git push: %w; stdout: %s; stderr: %s", err, stdout, stderr)
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func parseSeedStrict() (bool, error) {
	value := os.Getenv("PROCESSGIT_SEED_STRICT")
	if value == "" {
		return true, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse PROCESSGIT_SEED_STRICT: %w", err)
	}
	return parsed, nil
}

func seedLogf(format string, args ...any) {
	log.Info("[seed] "+format, args...)
}

func logSeedRuntime() {
	seedLogf("Runtime identity: uid=%d gid=%d user=%s", os.Geteuid(), os.Getegid(), os.Getenv("USER"))
	seedLogCommand("Templates owner dir", "ls", "-ld", "/data/git/repositories", "/data/git/repositories/processgit-templates")
}

func seedLogCommand(label, name string, args ...string) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		seedLogf("%s command failed: %s %v err=%v", label, name, args, err)
	}
	if len(output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line != "" {
				seedLogf("%s output: %s", label, line)
			}
		}
	}
}

func ensureGitWorkTree(ctx context.Context, workDir string) error {
	stdout, stderr, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments("--is-inside-work-tree").
		WithDir(workDir).
		RunStdString(ctx)
	if err != nil || strings.TrimSpace(stdout) != "true" {
		if err != nil {
			log.Error("[seed] git rev-parse failed: stdout=%s stderr=%s", stdout, stderr)
			return fmt.Errorf("seed workDir is not a git work tree: %q err=%w; stdout: %s; stderr: %s", stdout, err, stdout, stderr)
		}
		return fmt.Errorf("seed workDir is not a git work tree: %q; stdout: %s; stderr: %s", stdout, stdout, stderr)
	}

	return nil
}

func ensureOriginRemote(ctx context.Context, workDir, repoPath string) error {
	stdout, stderr, err := gitcmd.NewCommand("remote").
		AddDynamicArguments("get-url", "origin").
		WithDir(workDir).
		RunStdString(ctx)
	if err != nil {
		if addStdout, addStderr, addErr := gitcmd.NewCommand("remote").
			AddDynamicArguments("add", "origin", repoPath).
			WithDir(workDir).
			RunStdString(ctx); addErr != nil {
			log.Error("[seed] git remote add failed: stdout=%s stderr=%s", addStdout, addStderr)
			return fmt.Errorf("git remote add: %w; stdout: %s; stderr: %s", addErr, addStdout, addStderr)
		}
		return nil
	}

	if strings.TrimSpace(stdout) == repoPath {
		return nil
	}
	if stdout, stderr, err = gitcmd.NewCommand("remote").
		AddDynamicArguments("set-url", "origin", repoPath).
		WithDir(workDir).
		RunStdString(ctx); err != nil {
		log.Error("[seed] git remote set-url failed: stdout=%s stderr=%s", stdout, stderr)
		return fmt.Errorf("git remote set-url: %w; stdout: %s; stderr: %s", err, stdout, stderr)
	}
	return nil
}

func ensureOriginVisible(ctx context.Context, workDir string) error {
	stdout, stderr, err := gitcmd.NewCommand("remote").
		AddDynamicArguments("-v").
		WithDir(workDir).
		RunStdString(ctx)
	if err != nil {
		log.Error("[seed] git remote -v failed: stdout=%s stderr=%s", stdout, stderr)
		return fmt.Errorf("git remote -v: %w; stdout: %s; stderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "origin") {
		return fmt.Errorf("git remote -v missing origin for workDir %q; stdout: %s; stderr: %s", workDir, stdout, stderr)
	}
	return nil
}
