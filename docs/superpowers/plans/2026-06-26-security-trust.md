# Security Trust Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Glade security proof gates, release provenance, SBOM output, and public trust docs for architects evaluating laptop installs.

**Architecture:** Keep product runtime unchanged. Add CI and release workflow gates around the existing Go build and docs pipeline, then publish the proof in `SECURITY.md`, repo docs, and the public VitePress guide.

**Tech Stack:** GitHub Actions, Go `govulncheck`, gosec SARIF, CodeQL, OpenSSF Scorecard, CycloneDX GoMod, GitHub artifact attestations, VitePress docs.

---

### Task 1: Add Tests For The Trust Surface

**Files:**
- Modify: `site/tests/theme.test.mjs`

- [x] **Step 1: Add assertions that fail until the security page, SECURITY.md, workflows, release attestations, SBOMs, and README links exist.**

- [x] **Step 2: Run `npm test --prefix site`.**

Expected: failure because `site/docs-src/guide/security-trust.md` and `SECURITY.md` do not exist yet.

### Task 2: Add Security CI

**Files:**
- Create: `.github/workflows/security.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`

- [x] **Step 1: Add a scheduled and PR security workflow for govulncheck, CodeQL, gosec SARIF upload, dependency review, and OpenSSF Scorecard.**

- [x] **Step 2: Upgrade Go setup steps to the patched Go release used by the vulnerability gate.**

- [x] **Step 3: Generate CycloneDX SBOMs and artifact attestations for release archives.**

### Task 3: Add Public Security Docs

**Files:**
- Create: `SECURITY.md`
- Create: `docs/SECURITY_TRUST.md`
- Create: `site/docs-src/guide/security-trust.md`
- Modify: `site/.vitepress/config.ts`
- Modify: `README.md`
- Modify: `docs/INSTALL.md`
- Modify: `site/docs-src/guide/installation.md`

- [x] **Step 1: Document supported versions, reporting, laptop behavior, network access, local storage, install verification, SBOMs, attestations, and security gates.**

- [x] **Step 2: Add the public docs route and README badge/link.**

### Task 4: Verify

**Files:**
- All changed files

- [x] **Step 1: Run `npm test --prefix site`.**

- [x] **Step 2: Run `go test ./...`.**

- [x] **Step 3: Run `govulncheck ./...` with the patched Go toolchain when available.**

- [x] **Step 4: Run a gosec SARIF smoke command against tracked Go package directories.**

- [x] **Step 5: Run `git diff --check`.**

## Verification Run

- `npm test --prefix site`
- `/Users/matt/sdk/go1.26.4/bin/go test ./...`
- `/Users/matt/sdk/go1.26.4/bin/go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `/Users/matt/sdk/go1.26.4/bin/go run github.com/securego/gosec/v2/cmd/gosec@latest -no-fail -fmt sarif -out /tmp/glade-gosec-exact.sarif ./...`
- `go run github.com/rhysd/actionlint/cmd/actionlint@latest -ignore 'create-github-app-token@v3'`
- `npm audit --omit=dev --audit-level=high` in `third_party/lwc`
- `npm audit --omit=dev --audit-level=high` in `contrib/vscode-glade`
- `VERSION=v0.2.7-test DIST_DIR=/tmp/glade-release-smoke PATH="/Users/matt/sdk/go1.26.4/bin:$PATH" scripts/release-build.sh`
- `git diff --check`
