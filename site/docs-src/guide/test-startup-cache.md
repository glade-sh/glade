# Test Startup Cache

`glade test` can reuse warmed project state between runs. That saves time on
large projects, but a stale cache can make tests pass or fail against the wrong
local org or compiled helpers.

## TL;DR

- Glade caches warmed project state at `.glade/test/startup.meta.json` plus a
  hashed payload.
- Clear it after branch switches, Glade upgrades, or confusing stale behavior.
- Use `--no-cache` once to confirm whether the cache is involved.
- Restart `glade test serve` if the warm server still has old state.

## What is cached

The disk cache stores harness work that happens **before** tests run:

- Local org state inferred from metadata and Apex references
- Compiled project helper methods, classes, and triggers

It does **not** store test results, per-test org clones, or git state.

## Where it lives

Under `.glade/test/` in the project root:

| File | Purpose |
| ---- | ------- |
| `startup.meta.json` | Cache header with freshness manifest and payload hash |
| `startup.payload.<sha256>.gob` | Gob-encoded org and compiled runtime payload |
| `serve.sock` | Unix socket when `glade test serve` is running |
| `serve.pid` | PID file for the test server |

Do not commit `.glade/`. `glade dap` uses a separate cache at
`.glade/dap/startup.json`.

## When it is created

`startup.meta.json` and `startup.payload.<sha256>.gob` are written after a
**cold harness build** completes: project load, org inference, helper
compilation, payload write, then an atomic header write. The first run on a
large project is slow; later runs can read the header first and load the payload
only when the manifest is fresh.

Skipped when `--no-cache` is set.

## When it is reused

On each run (unless `--no-cache` or a running test server handles the request),
Glade reads `startup.meta.json` and checks **freshness**:

- Cache format version matches (currently **4**), along with manifest schema,
  platform ABI, and test-runtime ABI.
- Project root and the canonical project digest match. The digest covers API
  version, namespace and remaps, package directories, managed dependencies,
  artifacts, package shims, and effective org-shape features.
- Reloading the project produces the exact recorded input set. Additions,
  deletions, and renames are stale.
- Every tracked file matches its recorded size, modification time, and SHA-256
  content hash. Size and time are early rejection checks, not acceptance by
  themselves.

If anything fails, Glade cold-builds and writes a new cache. Corrupt or
mismatched payloads are ignored. A legacy `startup.gob` is read only when no
split header exists.

## Trust and recovery

Cache acceptance uses both the rebuilt input set and content hashes. New,
deleted, renamed, or changed tracked inputs produce a safe miss. Corrupt or
incompatible headers and payloads also produce a cold rebuild.

`glade test serve` keeps separate in-memory state after `clear-cache`; restart
the server when that warm process must reload. Glade upgrades that change the
cache, runtime, or platform ABI also produce a safe miss.

If tests act like an old project is still loaded, use `--no-cache` once, then
`glade test clear-cache`.

## Commands

```bash
glade test clear-cache --project .
glade test --project . --no-cache --class MyTest
glade test serve --project .
glade test daemon status --project .
glade test daemon stop --project .
glade test --project . --wizard
```

| Situation | Action |
| --------- | ------ |
| After `git pull` or branch switch | `clear-cache` |
| After upgrading Glade | `clear-cache` |
| Stale harness / missing new code | `--no-cache`, then `clear-cache` |
| `clear-cache` but serve still warm | `glade test daemon stop`, then start `glade test serve` again |

`--no-cache` skips disk read/write and does not auto-connect to a test server.

See the repository doc `docs/TEST_STARTUP_CACHE.md` for the full manifest rules
and serve/watch behavior.
