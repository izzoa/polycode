# Release Instructions (for LLM agents)

Step-by-step instructions for building, testing, and releasing polycode. Follow these exactly.

## Pre-Release Checklist

Before any commit or release, run these checks:

```bash
# 1. Verify everything compiles
go build ./...

# 2. Run tests with race detector
go test ./... -count=1 -race

# 3. Verify no uncommitted changes you didn't intend
git status
git diff --stat
```

All three must pass before proceeding.

### Update CHANGELOG.md and README.md

Before every release (and ideally with each commit that adds user-visible changes):

1. **CHANGELOG.md** — Add an entry for the new version under `## [Unreleased]` (or the version heading if tagging now). Include a brief summary of what changed, grouped by Added/Changed/Fixed/Removed. Move `[Unreleased]` entries under the new version heading when cutting a release.
2. **README.md** — Update any sections affected by the changes (features, configuration options, examples, architecture diagram, etc.). If a new feature was added, it should be documented in the README before the release goes out.

Do not tag a release until both files are up to date and committed.

## Routine Commit & Push

For regular feature/fix work:

```bash
# Stage specific files (never use git add -A or git add .)
git add <file1> <file2> ...

# Commit with a descriptive message via heredoc
git commit -m "$(cat <<'EOF'
Short summary of what changed

Optional longer description of why, if not obvious from the diff.
EOF
)"

# Push
git push
```

CI runs automatically on push to `main` — it will test, lint, and cross-compile.

## Checking CI Status

After pushing, verify CI passes:

```bash
# Check the latest workflow run
gh run list --limit 3

# Watch a specific run
gh run watch

# View details if it failed
gh run view <run-id> --log-failed
```

If CI fails, fix the issue and push again. Do not tag a release until CI is green.

## Cutting a Release

Only do this when explicitly asked. Releases trigger GoReleaser which builds cross-platform binaries and creates a GitHub Release.

### 1. Decide version number

Follow semver:
- **Patch** (`v0.1.1`) — bug fixes, minor tweaks
- **Minor** (`v0.2.0`) — new features, non-breaking changes
- **Major** (`v1.0.0`) — breaking changes, major milestones

Check the latest tag:

```bash
git tag --sort=-version:refname | head -5
```

If no tags exist, start with `v0.1.0`.

### 2. Verify main is clean and CI is green

```bash
# Ensure you're on main with no uncommitted changes
git checkout main
git pull
git status

# Run full checks locally
go build ./...
go test ./... -count=1 -race

# Confirm latest CI passed
gh run list --limit 1
```

### 3. Create and push the tag

```bash
git tag v<VERSION>
git push origin v<VERSION>
```

Example:
```bash
git tag v0.1.0
git push origin v0.1.0
```

### 4. Monitor the release workflow

```bash
gh run watch
```

The release workflow will:
1. Run tests (with race detector)
2. Run GoReleaser, which:
   - Builds binaries for linux/darwin/windows (amd64 + arm64)
   - Creates archives (tar.gz for unix, zip for windows)
   - Generates checksums
   - Creates a GitHub Release with all artifacts and a changelog

### 5. Verify the release

```bash
# Check the release was created
gh release view v<VERSION>

# List release assets
gh release view v<VERSION> --json assets --jq '.assets[].name'
```

Expected assets:
- `polycode_<version>_darwin_amd64.tar.gz`
- `polycode_<version>_darwin_arm64.tar.gz`
- `polycode_<version>_linux_amd64.tar.gz`
- `polycode_<version>_linux_arm64.tar.gz`
- `polycode_<version>_windows_amd64.zip`
- `checksums.txt`

## Understanding CI Artifacts vs Releases

**CI artifacts** (from the CI workflow on every push) are temporary build outputs:
- Found at: Actions → CI run → Summary → Artifacts (bottom of page)
- These are raw binaries, not versioned packages
- They expire after 90 days
- They do NOT appear in the "Releases" or "Packages" sidebar on the repo

**GitHub Releases** (from the Release workflow on version tags) are permanent, versioned distributions:
- Found at: the repo's "Releases" sidebar, or `https://github.com/<owner>/polycode/releases`
- Created by GoReleaser with proper archives (tar.gz/zip), checksums, and changelogs
- These are what users download to install polycode
- Only created when you push a `v*` tag (e.g., `git tag v0.1.0 && git push origin v0.1.0`)

**If the repo says "No releases published"** — you need to cut a release by tagging. CI passing alone does not create a release.

## Hotfix Release

If a release has a critical bug:

```bash
# Fix the bug on main
# ... make changes ...
go test ./... -count=1 -race
git add <files>
git commit -m "Fix: description of the bug fix"
git push

# Wait for CI to pass
gh run watch

# Tag the patch release
git tag v<MAJOR>.<MINOR>.<PATCH+1>
git push origin v<MAJOR>.<MINOR>.<PATCH+1>
```

## Troubleshooting

### CI lint failure
```bash
# Run the linter locally to see what it reports
golangci-lint run ./...
```

### GoReleaser failure
```bash
# Test GoReleaser locally without publishing
goreleaser build --snapshot --clean
```

### Test failure with race detector
Race conditions only surface with `-race`. Run the failing test in isolation:
```bash
go test -race -run TestName ./internal/package/
```
