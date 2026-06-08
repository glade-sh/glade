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

## GitHub Pages

The workflow at `.github/workflows/pages.yml` deploys this folder:

```
site/.vitepress/dist  -> /
site/install.sh       -> /install.sh
site/CNAME            -> custom domain
```

GitHub repo settings:

1. Open **Settings → Pages**.
2. Set **Build and deployment** to **GitHub Actions**.
3. Keep custom domain as `glade.sh`.

## Install Script

`site/install.sh` is served at `https://glade.sh/install.sh`.

```bash
curl -fsSL https://glade.sh/install.sh | sh
```
