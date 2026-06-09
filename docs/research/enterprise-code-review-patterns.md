# Enterprise Code Review Patterns (from sf-cred-pkg-develop)

> Analysis of 2,162 Apex classes, 41 triggers in a managed-package ISV product
> (namespace `verifiable`, AppExchange-listed). These patterns are NOT covered
> by PMD, Code Analyzer, or any existing static analysis tool. Each finding is
> based on a real bug or anti-pattern found in this codebase.

---

## Finding 1: Empty Catch Swallowing Initialization Failures

**Real bug:** `ProviderSpecialtySelector.cls:23` — `catch (Exception e) {}` after
initializing required fields from `SchemaService`. On exception, `wrapper`,
`providerLookupField`, and `mapping` stay null. Every method on the class
dereferences them in SOQL, producing a latent `NullPointerException` with zero
diagnostic context.

**Why PMD misses this:** PMD's `EmptyCatchBlock` only catches completely empty
blocks (`catch (Exception e) {}` on a single line). The real pattern is more
subtle: the block is syntactically non-empty (contains a comment or a nil
statement) but functionally empty — the exception is neither logged,
notified, rethrown, nor handled.

**What glade should detect:**
- Any catch block where the exception variable is unused (not passed to a
  logging call, not rethrown, not stored in a field, not included in an error
  message).
- Special severity when the catch block protects field/constructor
  initialization (type resolution can tell if the catch is in a constructor).
- Flag pattern: `catch (Exception e) { /* nothing that references 'e' */ }`

**Detection approach:**
- Walk AST for `try_statement` → `catch_clause`.
- In each catch body, search for references to the caught variable name.
- If no references found → finding `perf.code.empty-catch` (severity: high if
  in constructor/init, medium otherwise).
- If the catch body contains only a comment → also flag.

**Seen in:** `ProviderSpecialtySelector.cls:23`, `BoardCertSpecialtyTriggerHandler.cls:116`

---

## Finding 2: Error Context String Mismatch

**Real bug:** `HospitalAffiliationTriggerHandler.cls:22-33` — the `onAfterInsert()`
method contains:
```apex
ErrorLogService.logErrorFromException(null,
    ORIGIN + ' - onAfterUpdate',  // BUG: should be 'onAfterInsert'
    e, JSON.serializePretty(Trigger.new));
```

Same file at line 105-118: `onBeforeDelete()` logs `'onAfterDelete'`. These
misdirect anyone searching error logs — they'll look in the wrong method.

**Why PMD misses this:** No existing rule checks that string literals in
logging calls match the enclosing method name.

**What glade should detect:**
- In calls to known logging methods (ErrorLogService.logErrorFromException,
  System.debug, Logger.log, Nebula Logger, custom log objects), extract the
  context/tag/prefix string argument.
- Compare against the enclosing method name.
- Flag when the context string contains a method name that differs from the
  actual method.

**Detection approach:**
- In `method_declaration`, note the method name.
- Walk for `method_invocation` nodes matching known logging patterns.
- Extract the second/third string argument (context parameter position varies
  by logging framework — make configurable or heuristic).
- If the string contains `onAfterUpdate` but the method is `onAfterInsert` →
  flag.

**Difficulty:** Medium. Requires knowing the logging framework's parameter
positions. Initial implementation can focus on the common `ErrorLogService`
and `System.debug` patterns.

**Seen in:** `HospitalAffiliationTriggerHandler.cls:28, 113`

---

## Finding 3: Duplicate Mock/Spec Identifiers

**Real bug:** `HospitalAffiliationsSelector.cls:73` and line 94 share the same
`mockId('HospitalAffiliationsSelector.selectByProviderVuid')` — two different
methods (`selectByProviderVuid` and `selectByProviderId`). If a test mocks one,
it inadvertently mocks the other. This can produce:
- False positives: test passes because it's using the wrong mock's data.
- False negatives: test fails because the mock doesn't have the right shape
  for the other query.

**Why PMD misses this:** This is specific to the `SOQL.of()` fluent builder
pattern used by this codebase. No existing tool understands custom mocking
frameworks.

**What glade should detect:**
- In string arguments to `mockId()` or similar registration calls, detect
  duplicate values.
- The mock ID should typically be unique per query. A duplicate is either a
  copy-paste error or intentional (two queries sharing a mock) — in either
  case, flag for review.

**Detection approach:**
- Walk AST for `method_invocation` nodes with method name `mockId`.
- Extract the string argument.
- Build a map of mockId → location.
- Flag any mockId that appears at multiple locations.

**Seen in:** `HospitalAffiliationsSelector.cls:73, 94`

---

## Finding 4: Debug-Only SOQL (waste during test runs)

**Real pattern:** `HospitalAffiliationUpsertSyncActionTest.cls:46`
```apex
system.debug([SELECT Status__c, StatusMessage__c, ObjectType__c, ObjectSubType__c FROM Webhook_Event__c]);
```
The SOQL executes, consumes a governor limit query, but its results go only
to `System.debug` — which may not even be captured. This is debris from
debugging sessions committed to the repo.

**Why PMD misses this:** PMD's `AvoidDebugStatements` catches `System.debug`
but doesn't distinguish between `System.debug('hello')` (cheap) and
`System.debug([SELECT ...])` (governor limit impact).

**What glade should detect:**
- `System.debug()` calls where any argument is a `query_expression` node
  (inline SOQL).
- Severity: low in test files (just waste), medium in production code.

**Seen in:** Multiple test files across the codebase.

---

## Finding 5: API Version Drift

**Real issue:** `sfdx-project.json` declares `sourceApiVersion: "64.0"` but
`CLAUDE.md` (the developer's instructions) says `API Version: 66.0`. This
indicates inconsistent version configuration across the project.

**Why PMD misses this:** No existing tool compares API versions across
configuration files and class metadata.

**What glade should detect:**
- Read `sourceApiVersion` from sfdx-project.json.
- Walk individual class `.cls-meta.xml` files for `<apiVersion>` overrides.
- Flag classes with `apiVersion` different from the project default.
- Flag when the project default is stale (more than 1 year behind current).

**Seen in:** sfdx-project.json vs CLAUDE.md version mismatch.

---

## Finding 6: Architecture Drift

**Pattern detection:** The codebase follows a consistent layered architecture:
- Trigger → 1-line delegate to handler
- Trigger Handler → extends BaseDomain, validation + sync
- Selector → fluent SOQL builder, no DML
- Service → orchestration, no raw SOQL (only via Selectors)

A scanner could verify this architecture is maintained across all 41 triggers
and 2,100+ classes:

- **Trigger → Handler**: Verify every trigger delegates to exactly one
  handler class (no inline logic).
- **Selector purity**: Verify Selector classes contain only SOQL (no DML,
  no describe, no async enqueue).
- **Service encapsulation**: Verify Service classes route all SOQL through
  Selector classes (no raw `[SELECT ...]` in services).
- **Handler base class**: Verify trigger handlers extend the framework base
  class, not `Triggers.Handler` directly.

**Why this matters for PRs:** When a junior dev adds a SOQL directly to a
Service class, the reviewer catches it. A scanner catches it automatically
before review.

---

## Summary: Net-New Capabilities

| Finding | Severity | PMD Covers? | Unique to glade? |
|---------|----------|-------------|-------------------|
| Empty catch swallowing init | High | Partial (`EmptyCatchBlock` catches literal empties only) | Yes — checks exception variable usage |
| Error context string mismatch | High | No | Yes — requires method-name-aware string analysis |
| Duplicate mock IDs | Medium | No | Yes — framework-aware pattern check |
| Debug-only SOQL | Low | No (`AvoidDebugStatements` is binary) | Yes — checks argument for query cost |
| API version drift | Low | No | Yes — cross-file metadata comparison |
| Architecture drift | Medium | No (PMD has `AvoidLogicInTrigger` but not layered arch checks) | Yes — requires call-site analysis |

These are all detectable with glade's existing AST + type system infrastructure.
They don't require a VM, runtime execution, or org connection. They're purely
static analysis patterns that human reviewers catch manually today.
