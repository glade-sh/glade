# Site

The Glade site is a VitePress app. The landing page and docs are a single build.

## Local Preview

```bash
cd site
npm ci
npm run dev
```

Open the URL printed by VitePress. The landing page is at `/` and the docs at
`/guide/overview`.

For a production build preview:

```bash
npm run build
npm run preview
```

## Checks and release proof

Use `npm test` for the fast source verification and unit-test loop. Run the
release-orchestrator regression tests when its behavior changes:

```bash
npm test
npm run test:release
```

`npm run release:check` records the source and build proof. It runs `verify`,
`test:unit`, and `build:site` once each, rejects source changes during the run,
and writes `.vitepress/release-check.json`:

```bash
npm run release:check
```

It does not run the built-output, rendered browser, or preview smoke checks.
After the release check builds the current source, run:

```bash
npm run check:built
npx playwright install chromium
CI=1 GLADE_SITE_PREBUILT=1 npm run test:browser
```

Keep port 4173 free for Playwright's preview. `CI=1` prevents reuse of an existing
server; otherwise local tests can silently inspect another checkout's preview.
Only set `GLADE_SITE_PREBUILT=1` after building the current source.

For the preview smoke, start `npm run preview -- --host 127.0.0.1 --port 4173`
from this checkout in a separate terminal, wait for its ready URL, then run:

```bash
npm run smoke:preview -- http://127.0.0.1:4173
```

Stop that preview when finished. These checks also run in the CI site job.
For the site, `scripts/release-check.sh` at the repo root runs only the source
and build proof.

## Cloudflare Pages

Connect the GitHub repository to Cloudflare Pages with Git integration.
Use these settings:

```
Project name: glade-site
Production branch: main
Root directory: site
Build command: npm run build
Build output directory: .vitepress/dist
```

The build publishes these files:

```
site/.vitepress/dist  -> /
site/install.sh       -> /install.sh
```

Cloudflare domain settings:

1. Open **Workers & Pages → glade-site → Custom domains**.
2. Add `glade.sh`.
3. Let Cloudflare create the DNS record for the apex domain.

## Install Script

`site/install.sh` is served at `https://glade.sh/install.sh`.

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

## Launch Smoke Check

Git integration should deploy `main`. If a manual production deployment is
needed, use a clean checkout of the intended `origin/main` commit and publish
that exact commit to the existing production project. From `site/`:

```bash
(
set -e
npm ci
git -C .. fetch origin main
release_sha="$(git -C .. rev-parse HEAD)"
test "$release_sha" = "$(git -C .. rev-parse origin/main)"
test -z "$(git -C .. status --porcelain --untracked-files=all)"
CF_PAGES_COMMIT_SHA="$release_sha" npm run build
npx --yes wrangler pages deploy .vitepress/dist --project-name glade-site --branch main \
  --commit-hash "$release_sha" --commit-dirty=false
)
```

After the production deployment, reconcile the deployed commit and stable
release using the standalone postdeploy smoke. It is not run by the Release
workflow. Run from the checkout of the deployed commit:

```bash
expected_sha="$(git -C .. rev-parse HEAD)"
npm run smoke:postdeploy -- --base-url https://glade.sh --expected-commit "$expected_sha"
```

The smoke check covers public routes, redirects, security and cache headers,
`/install.sh`, `/site-build.json`, the stable release manifest, GitHub latest,
checksums, release assets, the plugin registry, and the sitemap.

## Brand and reading system

The approved dark homepage lives in `.vitepress/theme/home/GladeHome.vue`.
Its fixed examples are illustrative; the website never executes visitor Apex.
The normal VitePress layout owns documentation, guides, reference, compatibility,
Help, and contributor pages. Article `pageType` and `canonicalTask` frontmatter
record the reading role and primary task without moving public URLs.

Shared light/dark colors and font roles live in
`.vitepress/theme/styles/tokens.css`; `styles/reading.css` maps reading density
to VitePress components. Inter and IBM Plex Mono are self-hosted dependencies.
The home uses its own dark tokens and does not write the docs appearance
preference. Large catalogs and demo modules load on demand; automatic link
prefetch is disabled so ordinary articles do not fetch them.

After source changes, rebuild before preview/browser checks and restart any
preview server that was started against an older build. For a separate local
review server:

```bash
npm run preview -- --host 127.0.0.1 --port 4174
```

The browser suite covers the approved home, command lookup, capability
pagination, search, appearance, code copying, and no-JavaScript reading.
Performance assertions retain the checked budget; attached metrics distinguish
asset transfer, LCP, layout shift, and main-thread blocking.
