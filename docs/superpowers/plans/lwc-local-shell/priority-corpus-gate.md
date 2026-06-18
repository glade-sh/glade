# Priority LWC Corpus Gate

Date: 2026-06-18
Branch: `codex/priority-lwc-dx`

This gate uses the private priority corpus only as verification material. Do not copy these project names into public docs or site pages.

## Corpus Root

Default root:

```bash
/Users/matt/.sf-repo-analysis/repos
```

Override root:

```bash
GLADE_LWC_CORPUS=/path/to/repos node scripts/dev/lwc-priority-corpus-audit.mjs
```

## Projects

- `src-nbm-solhub-develop`
- `src-nmb-namz-prog-develop`
- `src-nmb-nc-develop`
- `src-nmb-nu-develop`
- `src-nmb-nudev-develop`
- `src-nmb-nuq-develop`
- `src-nmb-nutpl-develop`
- `src-nmb-nutplx-master`
- `sf-cred-pkg-develop`

## Static Audit

```bash
node scripts/dev/lwc-priority-corpus-audit.mjs
```

Expected shape:

- Five projects report nonzero `lwcFiles`.
- Four projects report zero `lwcFiles`.
- Missing local repo directories report `exists: false` and zero counts.

## Local Server Smoke

Short run:

```bash
node scripts/dev/lwc-priority-corpus-smoke.mjs
```

Hold servers for browser smoke:

```bash
node scripts/dev/lwc-priority-corpus-smoke.mjs --hold
```

Root-page browser smoke:

```bash
node scripts/dev/lwc-priority-browser-smoke.mjs \
  http://127.0.0.1:18080/ \
  http://127.0.0.1:18081/ \
  http://127.0.0.1:18082/ \
  http://127.0.0.1:18083/ \
  http://127.0.0.1:18084/
```

Generated route smoke:

```bash
node scripts/dev/lwc-priority-browser-smoke.mjs --discover \
  http://127.0.0.1:18080/ \
  http://127.0.0.1:18081/ \
  http://127.0.0.1:18082/ \
  http://127.0.0.1:18083/ \
  http://127.0.0.1:18084/
```
