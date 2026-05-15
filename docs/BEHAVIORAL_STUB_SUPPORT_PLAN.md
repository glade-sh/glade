# Behavioral Stub Support Plan

This plan tracks the move from stub shape parity to Salesforce-like behavior for
generated Apex platform types and standard SObject metadata. The source contract
is `example-projects/stubs`; behavior must be implemented from public Salesforce
docs, owned compatibility fixtures, or black-box scratch-org probes.

## Current Baseline

The declaration breadth gate is `oaer compat stub-inventory --json`.

Current checked baseline:

- System/product stub source types missing generated OAER types: 0
- SObject stub source objects missing active OAER objects: 0
- SObject stub fields missing after supported feature gates: 0
- SObject stub fields missing only because their org feature gate is disabled
  remain reported separately from unsupported field gaps.
- Remaining work is behavioral support, not type-shape coverage.

## Phase 1: Behavior Inventory Gate

Deliverable: `oaer compat stub-behavior --json`.

Track every generated constructor, method, and property with one status:

- `implemented`: local behavior is intentionally modeled and covered.
- `captured-side-effect`: local runtime records a test-visible side effect.
- `passive-default`: local runtime returns a deterministic typed value or DTO.
- `unsupported`: local runtime returns a stable unsupported diagnostic.
- `unknown`: behavior has not been classified.

Required output:

- Totals by status.
- Totals by namespace and class.
- Samples of unknown/passive/unsupported members.
- A check mode suitable for CI drift detection.

Promotion rule: no API moves to `implemented` without a unit test or compat
fixture. Ambiguous Salesforce behavior needs a scratch-org probe.

## Phase 2: Passive DTO and Value Objects

Target generated classes that are data carriers:

- Zero-arg and generated constructor support.
- Named-field constructor binding.
- Generated property default initialization.
- Case-insensitive property get/set.
- Getter/setter aliases where the stub shape makes them obvious.
- Deterministic `equals`, `hashCode`, and `toString` where local object identity
  semantics are already established.

Exit gate:

- `stub-behavior` shows DTO classes classified as `passive-default` or
  `implemented`, not `unknown`.
- Focused VM tests cover generated construction, named properties, and typed
  getter/setter round-trips.

## Phase 3: Core System Behavior

Prioritize high-use Apex runtime primitives:

- `System`, `Assert`, `Test`, `Limits`, `UserInfo`
- `String`, `Pattern`, `Matcher`, `Blob`, `EncodingUtil`, `Crypto`
- `Date`, `Datetime`, `Time`, `TimeZone`, `Math`, `Decimal`
- `Type`, `JSON`, `JSONParser`, `JSONGenerator`

Exit gate:

- Existing handwritten stdlib behavior is represented in `stub-behavior`.
- Unknown generated overloads are either implemented, passive, or explicit
  unsupported.
- Scratch-org probes exist for ambiguous date/time, regex, crypto, and JSON
  edge behavior.

## Phase 4: Schema and Describe Behavior

Implement describe surfaces from active metadata:

- `Schema.SObjectType`
- `Schema.SObjectField`
- `Schema.DescribeSObjectResult`
- `Schema.DescribeFieldResult`
- Record types, picklists, references, child relationships, and feature-gated
  fields.

Exit gate:

- `stub-inventory` remains zero-missing under supported feature gates.
- Describe fixtures cover standard objects, generated overlay-only objects, and
  custom metadata/custom object shapes.

## Phase 5: Test Runtime and Captured Side Effects

Implement or capture test-visible behavior:

- `Test.startTest` / `Test.stopTest`
- async queueables, futures, batches, schedulables
- callout mocks and web service mocks
- email sends
- platform event publishing
- files/content/document writes
- platform cache basics

Exit gate:

- Side effects are inspectable in local test result state.
- Unsupported external delivery stays explicit.

## Phase 6: UI and Controller APIs

Implement test-facing controller surfaces:

- `ApexPages`
- `PageReference`
- standard controllers and standard set controllers
- Visualforce page metadata and `Page.*`
- Site, Network, and Auth context where tests can observe it.

Exit gate:

- Controller tests run without rendering UI.
- Page rendering APIs remain explicit unsupported unless modeled.

## Phase 7: Product Namespace Behavior

Prioritize by example-project and stub inventory usage:

- `ConnectApi`
- `Metadata`
- `Invocable`
- `ApexPages`
- Commerce/payment/product namespaces used by fixtures.

Default policy:

- DTO classes: passive or implemented value behavior.
- Service calls: implemented only when deterministic local semantics exist.
- Cloud-only calls: stable unsupported diagnostics.

Exit gate:

- No `unknown` behavior remains in prioritized namespaces.

## Phase 8: Scratch-Org Probe Harness

Use `oaer-probe-lab` / `oaer-probe-org` for ambiguous behavior:

```bash
echo "System.debug('probe');" | sf apex run --target-org oaer-probe-org
```

Deliverables:

- Probe scripts or generated Apex classes grouped by API family.
- Captured expected outputs in repo-owned fixtures.
- Local equivalents that assert the same observable behavior or an explicit
  unsupported diagnostic.

## Phase 9: Promotion Gates

Add release-ready gates:

- `oaer compat stub-inventory --check ...`
- `oaer compat stub-behavior --check ...`
- required core APIs must be `implemented` or explicitly `unsupported`
- no `unknown` behavior in core namespaces
- no silent passive-default behavior for side-effecting service calls

Final completion criteria:

- Every generated stub member is classified.
- Every core/system behavior is implemented or explicitly unsupported.
- Every generated SObject object/field/relationship is represented in active
  metadata under the correct feature gate.
- Every supported behavior has local tests and, where needed, scratch-org
  evidence.
