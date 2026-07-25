# Site

The Glade site is a VitePress app. The landing page and docs are a single build.

## Local Preview

```bash
cd site
npm ci
npm run dev
```

Open the URL printed by VitePress. The landing page is at `/` and the docs at `/guide/`.

For a production build preview:

```bash
npm run build
npm run preview
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
npm run build
npx --yes wrangler pages deploy .vitepress/dist --project-name glade-sh --branch main \
  --commit-hash "$(git -C .. rev-parse HEAD)" --commit-dirty=false
```

After the production deployment, replace `vX.Y.Z` below with the release being
published and verify the rendered release and registry copy as well as the
public routes:

```bash
expected_sha="$(git -C .. rev-parse --short=7 HEAD)"
actual_sha="$(npx --yes wrangler pages deployment list --project-name glade-sh \
  --environment production --json | jq -r '.[0].Source')"
test "$actual_sha" = "$expected_sha"

cache_bust="$(date +%s)"
curl -fsSL "https://glade.sh/install.sh?v=$cache_bust" | head -n 5
curl -fsSI "https://glade.sh/install.sh?v=$cache_bust" | grep -i content-type
curl -fsSL "https://glade.sh/guide/support-map?v=$cache_bust" >/dev/null
curl -fsSL "https://glade.sh/?v=$cache_bust" | grep -F 'Latest stable release:<span class="home-release-version">vX.Y.Z</span>'
curl -fsSL "https://glade.sh/guide/plugins/first-party?v=$cache_bust" | grep -F 'https://plugins.glade.sh/index.json'
curl -fsSL "https://glade.sh/guide/local-testing?v=$cache_bust" | grep -F -- '--cpu-profile'
curl -fsSL "https://glade.sh/guide/local-testing?v=$cache_bust" | grep -F -- '--mem-profile'
curl -fsSL "https://glade.sh/guide/local-testing?v=$cache_bust" | grep -F -- '--perf-json'
curl -fsSL "https://glade.sh/guide/local-testing?v=$cache_bust" | grep -F 'do not replace Salesforce validation'
```

`/install.sh` must return shell script text, not the legacy project HTML.
