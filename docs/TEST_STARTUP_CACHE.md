# Test Startup Cache

`glade test` can reuse warmed project state between runs. That saves time on
large projects, but a stale cache can make tests pass or fail against the wrong
local org or compiled helpers. Treat the cache as a performance optimization,
not a source of truth.

## What gets cached

The disk cache stores the expensive **harness** work that happens before your
tests execute:

| Cached | Not cached |
| ------ | ---------- |
| Local org state built from project metadata and Apex references | Per-test org clones |
| Inferred standard fields, record types, and related org shape | Test results or reports |
| Compiled project helper methods, classes, and triggers | Git state or branch selection |
| Visualforce page names registered on the base VM | Whether a test server is running |

Test methods themselves are compiled separately per run. The cache only affects
how long you wait before the first test starts.

## Where it lives

All test-runtime artifacts live under `.glade/test/` in the project root:

| File | Purpose |
| ---- | ------- |
| `startup.meta.json` | Cache header with freshness manifest and payload hash |
| `startup.payload.<sha256>.gob` | Gob-encoded org and compiled runtime payload |
| `serve.sock` | Unix socket for `glade test serve` (when a server is running) |
| `serve.pid` | PID file for the running test server |

`.glade/` is gitignored by default. Do not commit these files.

`glade dap` uses a **separate** cache at `.glade/dap/startup.json`. Clearing or
changing the test cache does not affect DAP.

## Three warm layers

Glade can keep state warm at three levels. They are related but independent.

1. **Disk cache (`startup.meta.json` plus payload)** — survives across CLI
   invocations and terminal restarts. Loaded on the next `glade test` when
   freshness checks pass.
2. **In-process cache** — lives inside a single `glade test`, `glade test serve`,
   or `glade test --daemon --watch` process. Dropped when the process exits.
3. **Test server (`glade test serve`)** — keeps layer 2 warm across separate CLI
   runs by auto-connecting through `serve.sock`.

A fast run might use disk cache only. A faster loop uses `glade test serve`. The
slowest path is a cold harness build with no disk hit.

## When the disk cache is created

`startup.meta.json` and `startup.payload.<sha256>.gob` are written **after** a
full cold harness build completes:

1. Load project, schema, and type index.
2. Build local org state (`orgFromIndex`) and compile project helpers.
3. Write the gob payload under its SHA-256 name.
4. Atomically write `.glade/test/startup.meta.json` (temp file + rename).

Creation is skipped when:

- `--no-cache` is set on the run.
- A fresh split cache was already loaded for this harness build (no rewrite on
  every run).

The first test run on a large project is therefore slow: it pays the cold cost
and writes the cache for later runs.

## When the disk cache is loaded

On each `glade test` run (unless `--no-cache` or a running test server handles
the request), Glade:

1. Reads `startup.meta.json` if it exists.
2. Verifies the header manifest is **fresh** (see below).
3. If fresh, checks the payload file name, size, and SHA-256 hash.
4. If fresh and verified, restores org state and compiled runtime and skips the
   cold harness.
5. If missing, corrupt, stale, wrong version, or hash-mismatched, performs a
   cold build and may write a new cache.

Legacy `startup.gob` files are read only when no split header exists. Corrupt
payloads or legacy gob files are ignored and trigger a cold rebuild.

## Freshness: how the cache stays up to date

Each cache entry carries a **manifest** of fingerprints recorded at write time.
On load, `Fresh()` requires:

- Cache format version matches (currently **2**).
- `projectRoot` matches the current `--project` path.
- Every **tracked project file** in the manifest still exists with the same
  size and modification time.
- Every **config file** in the manifest still exists with the same size and
  modification time.
- Every **package root directory** in the manifest still exists with the same
  modification time.

### Tracked project files

The manifest includes paths from the loaded project model, including Apex
classes and triggers, object and field metadata, profiles, permission sets,
managed-package dependency roots, and other metadata files Glade indexes.

### Tracked config files

- `sfdx-project.json`
- `glade.yml`
- `config/project-scratch-def.json`
- `config/hc-project-scratch-def.json`
- `cumulusci.yml`
- `cumulusci.template.yml`

### Package roots

Package directory entries from `sfdx-project.json` are tracked by directory
modification time. Adding or removing files under a package usually updates the
parent directory mtime and invalidates the cache.

## When the cache can be wrong

Fingerprinting is fast but not perfect. Know these gaps:

**New files not yet in the manifest.** The cache only checks files that were
present when it was written. A newly added Apex class or metadata file may not
invalidate an existing cache until something else triggers a cold build (for
example, a package-root mtime change or an edit to a file already in the
manifest).

**Deleted tracked files.** Missing project files that were in the manifest
invalidate the cache. Deleting a tracked Apex class, trigger, or metadata file
forces a cold harness build before tests run.

**Running test server after `clear-cache`.** `glade test clear-cache` removes
the header, payload, and legacy gob files. A `glade test serve` process that is
already running keeps its in-memory warm state until you restart it.

**Glade upgrades.** A new Glade release may change org inference or compilation
without bumping the cache format version. After upgrading Glade, clear the cache
if harness behavior looks wrong.

**Unusual filesystem behavior.** Tools that preserve mtimes across checkouts or
copies can delay invalidation. When in doubt, clear the cache.

If tests behave as if an old version of the project is still loaded — missing
new classes, stale org fields, or failures that disappear with `--no-cache` —
treat the cache as suspect.

## Commands

```bash
# Delete the on-disk startup cache
glade test clear-cache --project .

# One run without reading or writing the startup cache (also skips test-server auto-connect)
glade test --project . --no-cache --class MyTest

# Force a local harness build even if a test server is running
glade test --project . --no-serve --class MyTest

# Persistent warm server (separate from the disk cache, but shares .glade/test/)
glade test serve --project .

# Inspect or stop the warm server
glade test daemon status --project .
glade test daemon stop --project .

# Print cache/server/next-command hints without running tests
glade test --project . --wizard
```

### When to clear or bypass

| Situation | Action |
| --------- | ------ |
| After `git pull`, branch switch, or large metadata import | `glade test clear-cache` |
| After upgrading Glade | `glade test clear-cache` |
| Harness looks stale; tests don't see new code | `--no-cache` once, then `clear-cache` |
| Debugging org inference or compile issues | `--no-cache` |
| `clear-cache` but serve still feels warm | `glade test daemon stop`, then start `glade test serve` again |

## Test server (`glade test serve`)

`glade test serve` warms the harness in a long-lived process and listens on
`.glade/test/serve.sock`. Later `glade test` runs auto-connect when the socket
is reachable.

With `--watch` (the default), the server watches project files. On Apex class or
trigger changes it reloads the index, drops in-process caches, and rebuilds the
warm runtime. That rebuild also writes a new split startup cache when the
harness completes.

`glade test serve --no-warm` skips the initial warm on startup. The first client
run pays the cold cost.

Use `--connect` to require the server. Use `--no-serve` to force a local build
even when a server is running.

## In-process warm (`--daemon`)

`glade test --daemon --watch` keeps the index and reference graph warm inside one
CLI process. It does not replace the disk cache or the test server. Use it for a
single long watch session; use `glade test serve` when separate terminal
invocations should stay warm.

## Related docs

- [LOCAL_TESTING.md](LOCAL_TESTING.md) — day-to-day test workflows
- [EDITOR.md](EDITOR.md) — editor tasks and DAP cache
