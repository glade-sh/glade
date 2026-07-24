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
2. Verifies the cache version, manifest schema, and platform ABI.
3. Reloads the project and verifies the header manifest is **fresh** (see below).
4. If fresh, checks the payload file name, size, and SHA-256 hash.
5. Verifies the test runtime ABI, then restores org state and compiled runtime
   and skips the cold harness.
6. If missing, corrupt, stale, wrong version, or hash-mismatched, performs a
   cold build and may write a new cache.

Legacy `startup.gob` files are read only when no split header exists. Corrupt
payloads or legacy gob files are ignored and trigger a cold rebuild.

## Freshness: how the cache stays up to date

Each cache entry carries a schema-versioned **manifest** of fingerprints
recorded at write time. The current cache version is **4** and the current
manifest schema is **1**. On load, `Fresh()` requires:

- Cache version, manifest schema, and platform ABI match the running Glade
  binary. The test runtime ABI is checked separately.
- `projectRoot` matches the current `--project` path.
- Reloading the project and rebuilding its trusted input set succeeds.
- The rebuilt set exactly matches the recorded project, dependency, package
  shim, artifact, and config paths. Additions, deletions, and renames are
  stale.
- The canonical project digest still matches. It covers API version,
  namespace, namespace remaps, package directories, managed dependencies,
  artifacts, package shims, and effective org-shape features.
- Every tracked file still has the recorded size and modification time. These
  are early rejection checks only. Acceptance also requires the recorded
  SHA-256 content hash to match.

### Tracked project files

The manifest includes paths from the reloaded project model, including Apex
classes and triggers, object and field metadata, profiles, permission sets,
recursive managed-package dependency projects, package-shim projects,
dependency artifacts, and other metadata files Glade indexes. It also includes
the inputs discovered directly by the test runtime:

- Optional `-meta.xml` sidecars for every known Apex class and trigger. These
  files select the API version used to compile project methods.
- `*.notiftype` and `*.notiftype-meta.xml` files below the project root. These
  files populate `CustomNotificationType` records.
- JSON files below eligible `data` directories in the main project and loaded
  direct managed dependencies. These files contribute inferred fields,
  relationships, and record types. Hidden directories and generated or
  dependency directories such as `node_modules`, `vendor`, `dist`, and `bin`
  are skipped, matching runtime discovery. Transitive dependency data is not
  scanned because the test runtime does not consume it.

Unrelated JSON files outside `data` directories are not cache inputs.

### Tracked config files

- `sfdx-project.json`
- `glade.yml`
- `config/project-scratch-def.json`
- `config/hc-project-scratch-def.json`
- `cumulusci.yml`
- `cumulusci.template.yml`
- JSON scratch definitions referenced by CumulusCI `config_file` entries

An optional config file is part of the exact set as soon as it exists. Adding a
new `glade.yml`, scratch definition, or CumulusCI input invalidates a cache that
was built without it.

### Package roots and project semantics

Package directory roots are rebuilt from the current project instead of being
trusted from the stored header. Relevant files below those roots are compared
as a deterministic sorted set. The manifest also records a canonical semantic
digest, so an effective API version, namespace, remap, dependency, artifact,
shim, package-directory, or feature change cannot reuse the old harness.

## Cache boundaries

The exact manifest closes the former size/mtime and stored-file-list gaps.
Same-size edits with preserved mtimes are rejected by content hash. Relevant
file additions, deletions, renames, and branch-like directory replacements are
rejected by rebuilding the current set.

**Running test server after `clear-cache`.** `glade test clear-cache` removes
the header, payload, and legacy gob files. A `glade test serve` process that is
already running keeps its in-memory warm state until you restart it.

**Glade upgrades.** Cache version, manifest schema, runtime ABI, and platform ABI
changes are safe misses. An old cache is rebuilt rather than decoded as current.

**Read or load errors.** An unreadable input, incomplete manifest, project-load
error, or hash error is a cache miss. Glade rebuilds instead of accepting
partial freshness evidence.

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
