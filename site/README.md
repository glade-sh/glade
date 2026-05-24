# Site Deployment

This folder is a static GitHub Pages site.

## Local Preview

Any static server works:

```bash
cd site
python3 -m http.server 4173
```

Then open `http://127.0.0.1:4173`.

## GitHub Pages

The workflow at `.github/workflows/pages.yml` deploys this folder.

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
