# nvcoretop CI and Release Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub Actions CI plus a version-tag-triggered Linux amd64 release workflow.

**Architecture:** Keep automation in two focused workflows: CI validates normal development changes, and Release verifies then packages a tagged version. The release workflow uses the existing `main.version` variable via Go linker flags and native GitHub CLI release commands.

**Tech Stack:** GitHub Actions, Go 1.24 from `go.mod`, `actions/checkout@v6`, `actions/setup-go@v6`, `gh release`.

---

## File Structure

- Create `.github/workflows/ci.yml`: CI workflow for push, pull request, and manual runs.
- Create `.github/workflows/release.yml`: tag/manual release workflow for Linux amd64 artifacts.
- Modify `README.md`: document the release process and tag convention.

---

### Task 1: Add CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create the workflow directory**

Run:

```bash
mkdir -p .github/workflows
```

Expected: directory exists.

- [ ] **Step 2: Add CI workflow YAML**

Create `.github/workflows/ci.yml` with:

```yaml
name: CI

on:
  push:
    branches:
      - main
  pull_request:
    branches:
      - main
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    name: Test and build
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Setup Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod

      - name: Check module tidiness
        run: go mod tidy -diff

      - name: Check whitespace
        run: git diff --check

      - name: Test
        run: go test ./...

      - name: Test with DCGM tag
        run: go test -tags dcgm ./...

      - name: Build
        run: go build ./cmd/nvcoretop
```

- [ ] **Step 3: Parse YAML locally**

Run:

```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/ci.yml")'
```

Expected: exit 0 with no output.

- [ ] **Step 4: Commit CI workflow**

Run:

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add test and build workflow"
```

Expected: commit succeeds.

---

### Task 2: Add Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Add release workflow YAML**

Create `.github/workflows/release.yml` with:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      tag:
        description: Version tag to release, for example v0.1.0
        required: true
        type: string

permissions:
  contents: write

concurrency:
  group: release-${{ github.event_name == 'workflow_dispatch' && inputs.tag || github.ref_name }}
  cancel-in-progress: false

jobs:
  release:
    name: Build Linux amd64 release
    runs-on: ubuntu-latest

    steps:
      - name: Resolve version
        id: version
        shell: bash
        run: |
          if [[ "${GITHUB_EVENT_NAME}" == "workflow_dispatch" ]]; then
            version="${{ inputs.tag }}"
          else
            version="${GITHUB_REF_NAME}"
          fi

          if [[ ! "${version}" =~ ^v[0-9]+(\.[0-9]+){1,2}([-+][0-9A-Za-z.-]+)?$ ]]; then
            echo "version tag must look like v0.1.0 or v0.1.0-rc.1: ${version}" >&2
            exit 1
          fi

          echo "version=${version}" >> "${GITHUB_OUTPUT}"

      - name: Checkout
        uses: actions/checkout@v6
        with:
          ref: ${{ steps.version.outputs.version }}

      - name: Setup Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod

      - name: Check module tidiness
        run: go mod tidy -diff

      - name: Check whitespace
        run: git diff --check

      - name: Test
        run: go test ./...

      - name: Test with DCGM tag
        run: go test -tags dcgm ./...

      - name: Build release binary
        env:
          CGO_ENABLED: "1"
          GOOS: linux
          GOARCH: amd64
          VERSION: ${{ steps.version.outputs.version }}
        run: |
          mkdir -p dist/nvcoretop_${VERSION}_linux_amd64
          go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
            -o "dist/nvcoretop_${VERSION}_linux_amd64/nvcoretop" \
            ./cmd/nvcoretop
          cp README.md LICENSE "dist/nvcoretop_${VERSION}_linux_amd64/"

      - name: Package release
        env:
          VERSION: ${{ steps.version.outputs.version }}
        run: |
          tar -C dist -czf "dist/nvcoretop_${VERSION}_linux_amd64.tar.gz" \
            "nvcoretop_${VERSION}_linux_amd64"
          cd dist
          sha256sum "nvcoretop_${VERSION}_linux_amd64.tar.gz" \
            > "nvcoretop_${VERSION}_linux_amd64.tar.gz.sha256"

      - name: Publish GitHub Release
        env:
          GH_TOKEN: ${{ github.token }}
          VERSION: ${{ steps.version.outputs.version }}
        run: |
          if gh release view "${VERSION}" >/dev/null 2>&1; then
            gh release upload "${VERSION}" \
              "dist/nvcoretop_${VERSION}_linux_amd64.tar.gz" \
              "dist/nvcoretop_${VERSION}_linux_amd64.tar.gz.sha256" \
              --clobber
          else
            gh release create "${VERSION}" \
              "dist/nvcoretop_${VERSION}_linux_amd64.tar.gz" \
              "dist/nvcoretop_${VERSION}_linux_amd64.tar.gz.sha256" \
              --title "nvcoretop ${VERSION}" \
              --generate-notes
          fi
```

- [ ] **Step 2: Parse YAML locally**

Run:

```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/release.yml")'
```

Expected: exit 0 with no output.

- [ ] **Step 3: Run local release build/package smoke test**

Run:

```bash
rm -rf dist
VERSION=v0.0.0-test
mkdir -p "dist/nvcoretop_${VERSION}_linux_amd64"
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "dist/nvcoretop_${VERSION}_linux_amd64/nvcoretop" ./cmd/nvcoretop
cp README.md LICENSE "dist/nvcoretop_${VERSION}_linux_amd64/"
tar -C dist -czf "dist/nvcoretop_${VERSION}_linux_amd64.tar.gz" "nvcoretop_${VERSION}_linux_amd64"
cd dist && sha256sum "nvcoretop_${VERSION}_linux_amd64.tar.gz" > "nvcoretop_${VERSION}_linux_amd64.tar.gz.sha256"
```

Expected: exit 0 and creates the tarball plus checksum.

- [ ] **Step 4: Clean local release smoke artifacts**

Run:

```bash
rm -rf dist
```

Expected: no `dist` directory remains.

- [ ] **Step 5: Commit release workflow**

Run:

```bash
git add .github/workflows/release.yml
git commit -m "ci: add tagged release workflow"
```

Expected: commit succeeds.

---

### Task 3: Document Release Process

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add Release section to README**

Insert this section before `## Development`:

````markdown
## Release

Releases are created from version tags that start with `v`.

```sh
git tag v0.1.0
git push origin v0.1.0
```

Pushing the tag runs the release workflow. The workflow verifies the project,
builds a Linux amd64 binary with the tag injected into `nvcoretop --version`,
packages `nvcoretop`, `README.md`, and `LICENSE`, then attaches the tarball and
SHA256 checksum to the GitHub Release.

For a local versioned build:

```sh
go build -trimpath -ldflags "-X main.version=v0.1.0" ./cmd/nvcoretop
```
````

- [ ] **Step 2: Verify README content**

Run:

```bash
sed -n '1,240p' README.md
```

Expected: README includes the new Release section before Development.

- [ ] **Step 3: Commit README release docs**

Run:

```bash
git add README.md
git commit -m "docs: document release process"
```

Expected: commit succeeds.

---

### Task 4: Final Verification and Push

**Files:**
- Verify all files changed by previous tasks.

- [ ] **Step 1: Run local verification**

Run:

```bash
go mod tidy -diff
git diff --check
go test ./...
go test -tags dcgm ./...
go build ./cmd/nvcoretop
```

Expected: all commands exit 0.

- [ ] **Step 2: Verify workflow YAML parses**

Run:

```bash
ruby -e 'require "yaml"; %w[.github/workflows/ci.yml .github/workflows/release.yml].each { |path| YAML.load_file(path) }'
```

Expected: exit 0 with no output.

- [ ] **Step 3: Inspect git history and status**

Run:

```bash
git status -sb
git log --oneline --decorate -6
```

Expected: worktree clean and recent commits include the spec, CI workflow,
release workflow, and release documentation.

- [ ] **Step 4: Push to origin**

Run:

```bash
git push origin main
```

Expected: push succeeds.
