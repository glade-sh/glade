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

`npm run release:check` is the exact site release proof. It runs `verify`,
`test:unit`, and `build:site` once each, rejects source changes during the run,
and writes `.vitepress/release-check.json`:

```bash
npm run release:check
```

## Cloudflare Pages

Connect the GitHub repository to Cloudflare Pages with Git integration.
Use these settings:

```
Project name: glade-sh
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

1. Open **Workers & Pages → glade-sh → Custom domains**.
2. Add `glade.sh`.
3. Let Cloudflare create the DNS record for the apex domain.

## Install Script

`site/install.sh` is served at `https://glade.sh/install.sh`.

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

## Launch Smoke Check

Git integration should deploy `main`. If it does not, build a clean local
`main` and publish that exact commit to the existing production project:

```bash
npm ci
release_sha="$(git -C .. rev-parse HEAD)"
CF_PAGES_COMMIT_SHA="$release_sha" npm run build
npx --yes wrangler pages deploy .vitepress/dist --project-name glade-sh --branch main \
  --commit-hash "$release_sha" --commit-dirty=false
```

After the production deployment, reconcile the deployed commit and stable
release with the same smoke check used by the release workflow:

```bash
expected_sha="$(git -C .. rev-parse HEAD)"
npm run smoke:postdeploy -- --base-url https://glade.sh --expected-commit "$expected_sha"
```

The smoke check covers public routes, redirects, security and cache headers,
`/install.sh`, `/site-build.json`, the stable release manifest, GitHub latest,
checksums, release assets, the plugin registry, and the sitemap.
