# Site and Documentation Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the August 12 site review as one proof-first user journey with release-coherent metadata, job-based navigation, layered capability truth, and targeted responsive/accessibility gates.

**Architecture:** Reuse VitePress, the checked stable release manifest, generated editor-support data, existing VS Code screenshots, and the current test matrix. Keep generated/enumerated truth in scripts and data; keep explanatory prose in Markdown.

**Tech Stack:** VitePress 2, Vue 3, Node.js 22 standard library, Playwright, existing CSS and generated support data.

---

### Task 1: Release and deployment truth

**Files:**
- Create: `site/release-manifest.json`
- Create: `site/scripts/sync-release-manifest.mjs`
- Create: `site/scripts/write-site-build.mjs`
- Modify: `site/package.json`
- Modify: `site/docs-src/index.md`
- Modify: `site/scripts/check-built-site.mjs`
- Modify: `site/scripts/postdeploy-smoke.mjs`
- Test: `site/tests/site-feedback.test.mjs`

- [ ] **Step 1: Write failing release-coherence tests**

Assert that homepage Markdown imports the checked manifest instead of embedding
`vX.Y.Z`, the build writes `site-build.json`, and post-deploy smoke compares the
site build, live manifest, GitHub latest tag, checksums, and manifest assets.

- [ ] **Step 2: Verify RED**

Run: `node --test site/tests/site-feedback.test.mjs`

Expected: FAIL because the manifest and build metadata scripts do not exist.

- [ ] **Step 3: Add the minimum generated release path**

Store the current stable manifest unchanged, import it from the homepage, and
write this build artifact after VitePress finishes:

```json
{
  "schemaVersion": 1,
  "siteCommit": "<CF_PAGES_COMMIT_SHA or GITHUB_SHA or local-preview>",
  "releaseVersion": "<release-manifest.version>",
  "builtAt": "<ISO timestamp>"
}
```

- [ ] **Step 4: Verify GREEN**

Run: `node --test site/tests/site-feedback.test.mjs && npm run build --prefix site && npm run check:built --prefix site`

Expected: PASS and `.vitepress/dist/site-build.json` identifies the build commit
and stable release.

### Task 2: First-run path and information architecture

**Files:**
- Modify: `site/.vitepress/config.ts`
- Modify: `site/docs-src/index.md`
- Modify: `site/docs-src/guide/installation.md`
- Modify: `site/docs-src/guide/quickstart.md`
- Modify: `site/docs-src/guide/workflows.md`
- Modify: `site/docs-src/guide/modules.md`
- Modify: `site/docs-src/guide/tester-field-guide.md`
- Modify: `site/docs-src/help/index.md`
- Create: `site/docs-src/help/troubleshooting.md`
- Modify: `site/routes.json`
- Test: `site/tests/site-feedback.test.mjs`

- [ ] **Step 1: Write failing journey and IA tests**

Assert `glade version` is the homepage install verification, every first-run
doctor invocation is `glade doctor --project .` after project setup, nav has four
text destinations plus a styled Install action, sidebar links are unique, Help
offers Task guides and Troubleshooting, and Advanced remains collapsed.

- [ ] **Step 2: Verify RED**

Run: `node --test site/tests/site-feedback.test.mjs`

Expected: FAIL on the current homepage sequence, duplicate sidebar entries, and
Guided help taxonomy.

- [ ] **Step 3: Apply the smallest content/IA edit**

Keep one canonical route for Workbench, plugins, and VS Code. Rename visible
internal taxonomy to user jobs. Add a single symptom-based troubleshooting hub
instead of scaffolding one page per symptom. Add the quickstart completion and
cleanup checkpoints.

- [ ] **Step 4: Verify GREEN**

Run: `npm test --prefix site`

Expected: all route, generated-data, screenshot, and unit contracts pass.

### Task 3: Homepage product proof and layered capability truth

**Files:**
- Modify: `site/docs-src/index.md`
- Modify: `site/docs-src/guide/workflows.md`
- Modify: `site/docs-src/guide/support-map.md`
- Create: `site/.vitepress/theme/GladeSupportExplorer.vue`
- Modify: `site/.vitepress/theme/index.ts`
- Modify: `site/.vitepress/theme/custom.css`
- Modify: `site/docs-src/public/css/home.css`
- Test: `site/tests/site-feedback.test.mjs`
- Test: `site/tests/browser/site.spec.ts`

- [ ] **Step 1: Write failing proof/filter tests**

Assert the homepage contains outcome-led copy, a checked VS Code screenshot with
task-focused alt text, four job cards, standardized status labels, and one
prominent boundary. Browser-test support search/status filtering and its live
result count.

- [ ] **Step 2: Verify RED**

Run: `node --test site/tests/site-feedback.test.mjs`

Expected: FAIL because the product image, workflow cards, and generated support
explorer are absent.

- [ ] **Step 3: Reuse checked assets and generated support data**

Use `/help/screenshots/run-one-apex-test-03-test-explorer.png` rather than adding
a new mockup. Flatten `editorSupportCatalog.receivers` inside one Vue component;
filter by query and status, show a bounded result list, and link to the complete
checked ledgers.

- [ ] **Step 4: Verify GREEN**

Run: `npm test --prefix site && npm run build --prefix site && npm run check:built --prefix site`

Expected: PASS with generated capability counts and accessible filter status.

### Task 4: Responsive and interaction regression gates

**Files:**
- Modify: `site/docs-src/public/css/home.css`
- Modify: `site/.vitepress/theme/custom.css`
- Modify: `site/tests/browser/site.spec.ts`
- Modify: `site/tests/browser/mobile.spec.ts`
- Modify: `site/tests/accessibility-contract.test.mjs`

- [ ] **Step 1: Add failing mobile and interaction assertions**

At 320 and 360 pixels, assert hero metrics and status rows wrap without viewport
overflow. Exercise keyboard focus for support/CLI filters, copy status, theme
switching, and mobile navigation.

- [ ] **Step 2: Verify RED**

Run: `npm run test:browser --prefix site -- --grep "feedback|support explorer|320|360"`

Expected: FAIL on the unimplemented selectors or narrow-layout contract.

- [ ] **Step 3: Add only the required CSS and semantics**

Use existing breakpoints, tokens, live regions, and native controls. Add no new
layout library or screenshot dependency.

- [ ] **Step 4: Run full verification**

Run:

```bash
git diff --check
npm test --prefix site
npm run build --prefix site
npm run check:built --prefix site
npm run test:browser --prefix site
go test ./internal/gladecli ./internal/repoguard
```

Expected: all commands exit `0`.

### Task 5: Review the exact diff

- [ ] Confirm every public example uses neutral Glade naming and no private
  corpus identifiers.
- [ ] Confirm the feedback checklist is either implemented or explicitly marked
  as deployment/measurement follow-up.
- [ ] Confirm no built files, ad-hoc captures, or dependency changes are included.
- [ ] Commit the verified branch without deploying it.
