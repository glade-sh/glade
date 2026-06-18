# Priority LWC Corpus Results

Date: 2026-06-18
Branch: `codex/priority-lwc-dx`
Commit: recorded in final worker response and `git log`

This file is internal verification material. Private corpus names may appear here. Public docs and site pages must not include them.

## Static Audit

Command:

```bash
node scripts/dev/lwc-priority-corpus-audit.mjs
```

Current expectation:

| Project | LWC files | Apex classes | Static resource files | Route smoke status | Failures |
| --- | ---: | ---: | ---: | --- | --- |
| `src-nbm-solhub-develop` | 28 | 16 | 0 | root page passed | none on root page |
| `src-nmb-namz-prog-develop` | 0 | 66 | 0 | not applicable | none expected |
| `src-nmb-nc-develop` | 138 | 2419 | 87 | root page passed | none on root page |
| `src-nmb-nu-develop` | 239 | 2899 | 157 | root page passed | none on root page |
| `src-nmb-nudev-develop` | 14 | 38 | 6 | root page passed | none on root page |
| `src-nmb-nuq-develop` | 0 | 126 | 11 | not applicable | none expected |
| `src-nmb-nutpl-develop` | 0 | 134 | 463 | not applicable | none expected |
| `src-nmb-nutplx-master` | 0 | 17 | 2 | not applicable | none expected |
| `sf-cred-pkg-develop` | 1005 | 2164 | 36 | root page passed | none on root page |

## Browser Smoke Commands

Start servers:

```bash
node scripts/dev/lwc-priority-corpus-smoke.mjs --hold
```

Smoke root pages:

```bash
node scripts/dev/lwc-priority-browser-smoke.mjs \
  http://127.0.0.1:18080/ \
  http://127.0.0.1:18081/ \
  http://127.0.0.1:18082/ \
  http://127.0.0.1:18083/ \
  http://127.0.0.1:18084/
```

Result on 2026-06-18: all five root pages passed.

Smoke generated routes:

```bash
node scripts/dev/lwc-priority-browser-smoke.mjs --discover \
  http://127.0.0.1:18080/ \
  http://127.0.0.1:18081/ \
  http://127.0.0.1:18082/ \
  http://127.0.0.1:18083/ \
  http://127.0.0.1:18084/
```

Result on 2026-06-18: not run in this worker lane. The two largest projects
produce hundreds of generated routes, and full route walking should run after
the server/resource/base-component/service squads land their pieces.

## Fixed Issues

- Added root landing scrape markers with `[data-glade-route-link]`.
- Added URL-addressable and quick-action routes to the generated workbench model.
- Added builder controls for target, component, app, object, record sample, Community site, console mode, form factor, state, and Flow input draft data.
