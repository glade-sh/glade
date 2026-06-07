# Site Deployment

This folder is a static GitHub Pages site.

## Hosted Shape

The site deploys as one GitHub Pages app:

- `/` is the Glade home page from `site/index.html`.
- `/docs/` is the VitePress documentation app from `site/docs-src`.
- `/install.sh` is the install script from `site/install.sh`.

## Local Preview

Install the site dependencies once:

```bash
cd site
npm ci
```

### Docs-only preview

Use this when you are editing documentation pages only:

```bash
npm run docs:dev
```

Open the URL printed by VitePress. The docs app is configured with `base: '/docs/'`.

### Deployment-accurate preview

Use this when you need to see both the home page and docs exactly as GitHub Pages assembles them:

```bash
npm run preview:pages
```

Then open:

```text
http://127.0.0.1:65110/
http://127.0.0.1:65110/docs/
```

The script builds the VitePress docs, assembles a temporary Pages artifact in
`/tmp/glade-site-pages`, and serves that artifact.

Override the preview directory, host, or port when needed:

```bash
GLADE_SITE_PREVIEW_DIR=/tmp/glade-site-preview GLADE_SITE_HOST=127.0.0.1 GLADE_SITE_PORT=4173 npm run preview:pages
```

## GitHub Pages

The workflow at `.github/workflows/pages.yml` deploys this folder.

The workflow builds the docs, then assembles the Pages artifact the same way as
`npm run preview:pages`:

```text
site/index.html          -> /
site/.vitepress/dist     -> /docs/
site/install.sh          -> /install.sh
site/CNAME               -> custom domain
```

GitHub repo settings:

1. Open **Settings → Pages**.
2. Set **Build and deployment** to **GitHub Actions**.
3. Keep custom domain as `glade.sh`.

`site/CNAME` is already set to `glade.sh`.

## Install Script

`site/install.sh` is served at:

```text
https://glade.sh/install.sh
```

The public install command is:

```bash
curl -fsSL https://glade.sh/install.sh | sh
```
