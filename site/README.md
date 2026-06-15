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

Connect the private GitHub repository to Cloudflare Pages with Git integration.
Use these settings:

```
Project name: glade
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

1. Open **Workers & Pages → glade → Custom domains**.
2. Add `glade.sh`.
3. Let Cloudflare create the DNS record for the apex domain.

## Install Script

`site/install.sh` is served at `https://glade.sh/install.sh`.

```bash
curl -fsSL https://glade.sh/install.sh | sh
```

## Launch Smoke Check

After Cloudflare Pages points `glade.sh` at this site, verify the public routes:

```bash
curl -fsSL https://glade.sh/install.sh | head -n 5
curl -fsSI https://glade.sh/install.sh | grep -i content-type
curl -fsSL https://glade.sh/guide/support-map >/dev/null
curl -fsSL https://glade.sh/ >/dev/null
```

`/install.sh` must return shell script text, not the legacy project HTML.
