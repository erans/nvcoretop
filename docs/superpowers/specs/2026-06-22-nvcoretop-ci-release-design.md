# nvcoretop CI and Release Automation Design

**Date:** 2026-06-22
**Status:** Approved for implementation

## Summary

Add GitHub Actions automation for two project lifecycle paths:

- Continuous integration on pushes and pull requests.
- Version-tagged releases that publish a Linux amd64 build artifact.

The workflows should be small, auditable, and aligned with the current project
scope. `nvcoretop` currently supports Linux NVIDIA systems through cgo-backed
NVML/DCGM bindings, so the release artifact is Linux amd64 only.

## Goals

- Run the normal Go test suite on every push and pull request to `main`.
- Run the DCGM-tagged test suite so build-tagged code remains compiled and
  covered in CI.
- Build the CLI binary in CI.
- Check module tidiness and whitespace before changes merge.
- On tags matching `v*`, build a Linux amd64 release artifact.
- Inject the tag version into the binary through the existing `main.version`
  variable.
- Attach a tarball and SHA256 checksum to a GitHub Release.

## Non-goals

- Cross-platform releases.
- Linux arm64 releases.
- Package manager publishing.
- Docker images.
- Real GPU smoke tests in GitHub-hosted CI.
- Automated semantic version calculation.

## Workflow Design

### CI

File: `.github/workflows/ci.yml`

Triggers:

- `push` to `main`
- `pull_request` targeting `main`
- manual `workflow_dispatch`

Job:

- Ubuntu latest runner.
- Read-only repository permissions.
- Checkout repository.
- Install Go from `go.mod`.
- Run:
  - `go mod tidy -diff`
  - `git diff --check`
  - `go test ./...`
  - `go test -tags dcgm ./...`
  - `go build ./cmd/nvcoretop`

### Release

File: `.github/workflows/release.yml`

Triggers:

- Push tags matching `v*`
- manual `workflow_dispatch` with a `tag` input for rebuilding an existing tag
  or creating a release for a specific version tag.

Job:

- Ubuntu latest runner.
- `contents: write` permission, limited to the release workflow.
- Checkout repository at the tag ref.
- Install Go from `go.mod`.
- Run the same core verification used by CI before releasing.
- Build `nvcoretop` for Linux amd64:
  - `GOOS=linux`
  - `GOARCH=amd64`
  - `CGO_ENABLED=1`
  - `-ldflags "-X main.version=${version}"`
- Package:
  - `nvcoretop`
  - `README.md`
  - `LICENSE`
- Create `nvcoretop_${version}_linux_amd64.tar.gz`.
- Create `nvcoretop_${version}_linux_amd64.tar.gz.sha256`.
- Create or update the GitHub Release for that tag with the tarball and
  checksum attached.

## Release Process

Users create a release by tagging a commit:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The tag triggers the release workflow. The workflow is responsible for
verification, binary build, packaging, checksum creation, and GitHub Release
asset upload.

## Error Handling

- If tests, build, tidy, or whitespace checks fail, CI and release fail.
- If a manually supplied release tag does not start with `v`, the workflow
  fails before building.
- If a release already exists for the tag, the workflow updates the attached
  assets using `gh release upload --clobber`.

## Testing

- Validate workflow YAML syntax by parsing it locally.
- Run the same local commands used by CI:
  - `go mod tidy -diff`
  - `git diff --check`
  - `go test ./...`
  - `go test -tags dcgm ./...`
  - `go build ./cmd/nvcoretop`
- Run the release build/package commands locally with a sample version string.
