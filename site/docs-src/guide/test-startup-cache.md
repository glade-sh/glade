# Test Startup Cache

`glade test` can reuse warmed project state between runs. That saves time on
large projects, but a stale cache can make tests pass or fail against the wrong
local org or compiled helpers.

## What is cached

The disk cache stores harness work that happens **before** tests run:

- Local org state inferred from metadata and Apex references
- Compiled project helper methods, classes, and triggers

It does **not** store test results, per-test org clones, or git state.

## Where it lives

Under `.glade/test/` in the project root:

| File | Purpose |
| ---- | ------- |
| `startup.gob` | On-disk startup cache |
| `serve.sock` | Unix socket when `glade test serve` is running |
| `serve.pid` | PID file for the test server |

Do not commit `.glade/`. `glade dap` uses a separate cache at
`.glade/dap/startup.json`.

## When it is created

`startup.gob` is written after a **cold harness build** completes: project
load, org inference, helper compilation, then an atomic write. The first run on
a large project is slow; later runs can load the gob instead.

Skipped when `--no-cache` is set.

## When it is reused

On each run (unless `--no-cache`), Glade reads `startup.gob` and checks
**freshness**:

- Cache format version matches (currently **2**)
- Project root matches
- Tracked project files match recorded size and modification time
- Config files (`sfdx-project.json`, `glade.yml`, scratch defs, `cumulusci.yml`,
  etc.) match
- Package root directories match recorded modification time

If anything fails, Glade cold-builds and writes a new cache. Corrupt gob files
are ignored.

## Trust: when the cache can lie

Fingerprinting is fast but imperfect:

- **New files** added after the cache was written may not invalidate it until
  another tracked path changes.
- **Deleted files** may not invalidate the cache by themselves.
- **`glade test serve`** keeps in-memory state after `clear-cache` until you
  restart the server.
- **Glade upgrades** may change behavior without bumping the cache version.

If tests act like an old project is still loaded, use `--no-cache` once, then
`glade test clear-cache`.

## Commands

```bash
glade test clear-cache --project .
glade test --project . --no-cache --filter MyTest
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
