# Architecture

`oaer` is organized as a set of narrow packages that can be tested separately
and composed by the CLI.

## Current Packages

- `cmd/oaer`: executable entry point.
- `internal/oaercli`: command routing and user-facing CLI behavior.
- `internal/apexast`: parser adapter and stable source model over the public
  `apexfmt` ANTLR parser.
- `internal/config`: `oaer.yml` discovery and parsing.
- `internal/diagnostic`: shared diagnostic model for parser, semantic analysis,
  runtime, and CLI.
- `internal/project`: SFDX package directory discovery and source file
  collection.
- `internal/schema`: Metadata API custom object and field model.
- `internal/typesys`: first symbol index for declarations, members, triggers,
  and schema objects.
- `internal/sema`: first semantic pass for known-type catalogs and declaration
  type-reference diagnostics.
- `internal/ir`: compact executable representation for the initial VM slice.
- `internal/vm`: minimal interpreter for anonymous Apex smoke execution,
  including primitives, simple collections, `System` assertions/debug, and
  instruction traces.
- `internal/apextest` and `internal/testreport`: minimal Apex test discovery,
  execution against the current VM subset, and console/JSON/JUnit reporting.
- `internal/sobject`: runtime SObject value and schema describe helpers.
- `internal/storage`: in-memory org/object/record model, fixture envelope,
  deterministic IDs, and cloneable transaction snapshots.
- `internal/soql`: simple in-memory SOQL parser and executor.
- `internal/dml`: in-memory DML insert/update/delete pipeline and rollback
  wrapper.
- `internal/compat`: compatibility fixture schema.

## Planned Runtime Pipeline

1. Load project configuration and Salesforce metadata.
2. Parse Apex source through `internal/apexast`.
3. Build symbols and resolve references through `internal/typesys`.
4. Type-check through `internal/sema`.
5. Lower checked code into `internal/ir`.
6. Execute with `internal/vm`, routing platform calls into dedicated packages.
7. Record diagnostics, traces, profiles, test reports, and compatibility
   results in stable machine-readable formats.

## Design Constraints

- Keep the parser behind an adapter so grammar dependencies can change.
- Attach source ranges early and preserve them through diagnostics and runtime
  traces.
- Return explicit unsupported-feature diagnostics instead of panicking.
- Keep Salesforce behavior claims tied to compatibility fixtures.
