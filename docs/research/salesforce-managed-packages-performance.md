# Salesforce Managed Packages & Performance - Research Report

*Generated: 2026-06-08*

---

## 1. Salesforce Managed Packages & ISV Package Management

### 1.1 Managed vs Unmanaged Packages

| Aspect | Managed Package | Unmanaged Package |
|--------|-----------------|-------------------|
| **Source Code** | Hidden from subscriber (compiled/obfuscated) | Fully visible and editable |
| **Upgradability** | Supports upgrades via new versions | No upgrade path; one-time deployment |
| **IP Protection** | Full - source is protected | None - code is freely viewable/modifiable |
| **Namespace** | Required - provides isolation | Optional |
| **License Management** | Built-in via LMA (License Management App) | Not applicable |
| **Default Sharing** | Classes default to `private`, API calls default to subscriber's user context | Standard Apex sharing behavior |
| **Deletion** | Components can only be deprecated, never truly deleted | Can be fully removed |
| **API Access** | Only `global` access modifier exposes to subscribers | All public methods accessible |
| **AppExchange Listing** | Required for AppExchange distribution | Not suitable for AppExchange |

### 1.2 ISV Package Lifecycle and Versioning

Managed packages follow a strict versioning lifecycle:

1. **Beta Version** - Development/testing, can be deleted, installable in sandboxes only
2. **Released Version** - Immutable once released, installable in production
3. **Patch Version** - Bug fixes only, no new features or schema changes
4. **Major/Minor Version** - Full feature releases

Version numbers follow `major.minor.patch` format. Once released, a version is permanent - you cannot delete or modify it. This creates significant pressure around `global` access modifiers because anything marked `global` in a released version must remain `global` forever (or be deprecated through a multi-version deprecation cycle).

### 1.3 How Managed Packages Affect Performance Differently Than Unmanaged Code

**Key differences:**

- **Namespace prefix overhead**: Every schema reference in managed package SOQL must be fully qualified with the namespace prefix (`namespace__FieldName__c`), adding slight string processing overhead
- **Cross-namespace SOQL**: Queries that span both managed and subscriber objects have to go through namespace resolution, adding latency
- **Governor limit sharing**: Managed package code runs in the same transaction context as subscriber code - they **share** governor limits. A managed package trigger consuming 80% of SOQL queries leaves only 20% for subscriber code
- **Feature flag checks**: Most ISV packages implement feature gates per license tier, adding per-transaction overhead for license validation
- **Dynamic SOQL/SOSL**: When a managed package uses dynamic queries, namespace prefix must be injected programmatically (adding `Schema.getNamespace()` or hardcoded prefix logic)
- **Describe calls**: Managed packages cannot rely on subscriber org metadata being present, requiring additional `Schema.describeSObjects()` calls to validate field/object existence

### 1.4 Package Namespace Isolation and Performance Implications

- Each managed package gets a unique namespace (e.g., `MyApp__`). This namespace serves as a logical boundary
- **SOQL in managed packages**: Field references always include the namespace prefix, which means queries are slightly longer strings to parse. The platform internally maps namespace-qualified field names to internal identifiers
- **Class/Trigger isolation**: Managed package Apex executes in the namespace's security context by default, with `without sharing` behavior
- **Record Type IDs**: Subscriber Record Type IDs differ from development org IDs. Managed packages must query Record Types by DeveloperName, not ID `AvoidHardcodingId` (PMD rule)
- **Static variable isolation**: Static variables in managed packages are isolated to the package namespace; subscriber code cannot access package static variables unless exposed through `global` methods

### 1.5 Protected/Global Access Modifiers and Performance

- **`global`**: The only modifier that exposes classes/methods to the subscriber org. Once a `global` class/method is released in a version, it cannot be removed or have its signature changed
  - PMD rule `AvoidGlobalModifier` (PMD 5.5.0) flags global classes with priority Medium (3), noting: "Global classes should be avoided (especially in managed packages) as they can never be deleted or changed in signature"
- **`protected`**: Visible within the package and to subclasses. Not accessible to subscribers directly
- **`public`**: Visible within the package namespace only. Subscribers cannot access `public` members of a managed package
- **`private`**: Default for managed packages. Most restrictive
- **Performance impact**: The access modifier level affects the virtual dispatch mechanism. `global` methods can be overridden by subscribers, requiring virtual method table lookups. `private` and `public` methods can be inlined or directly dispatched

### 1.6 2GP (Second Generation Packaging) vs 1GP Performance Considerations

| Aspect | 1GP (First Generation) | 2GP (Second Generation) |
|--------|----------------------|-------------------------|
| **Metadata format** | Metadata API format | Source format (SFDX) |
| **Namespace** | Created in separate dev org | Namespace org or no namespace |
| **Version creation** | Upload from packaging org | CLI: `sf package version create` |
| **Dependencies** | Extension packages | `dependencies` in sfdx-project.json |
| **Scratch org development** | Limited | Full support |
| **Unlocked Packages** | N/A | Supports unlocked (non-managed) packages |
| **Source of truth** | Packaging org | Version Control System |
| **CI/CD** | Complex, org-dependent | Designed for CI/CD pipelines |

2GP performance considerations:
- Better dependency management means fewer dependency conflicts
- Namespace-free unlocked packages avoid namespace prefix overhead
- Code Analyzer integration is first-class in SFDX/2GP workflows

### 1.7 Package Dependency Management and Performance

- **Transitive dependencies** in 2GP can amplify governor limit consumption across multiple packages
- **Cross-package triggers** execute in order of package installation, affecting order-of-operations and cumulative limit consumption
- **Apex Language Server (apex-ls)** supports multi-package dependency analysis with `dependencies` in sfdx-project.json:
  ```json
  {
    "plugins": {
      "dependencies": [
        {"namespace": "aa"}
      ]
    }
  }
  ```
- Each dependency adds another namespace's governor limit consumption to the transaction pool

---

## 2. Performance Issues Specific to Managed Packages

### 2.1 Known Performance Pitfalls in Managed Packages

1. **Excessive describe calls**: Managed packages often defensively call `Schema.describeSObjects()` to verify subscriber org metadata. PMD rule `EagerlyLoadedDescribeSObjectResult` (PMD 6.40.0) flags describe calls without `SObjectDescribeOptions.DEFERRED`, noting the performance impact of eager loading child relationships
2. **Hardcoded ID references**: IDs change between environments. PMD rule `AvoidHardcodingId` (PMD 6.0.0) catches this pattern. Managed packages must query by DeveloperName
3. **Dynamic SOQL namespace injection**: Every dynamic query must inject the namespace prefix, adding CPU overhead
4. **Over-use of `global`**: Each `global` method creates an API surface that cannot be changed, leading to wrapper methods and indirection over time
5. **Cross-namespace cache misses**: Platform Cache keys scoped to namespace may cause repeated cache population if not designed properly
6. **Stateful batch issues**: PMD rule `AvoidStatefulDatabaseResult` (PMD 7.11.0) flags a managed-package-relevant bug where storing `Database.SaveResult` in stateful batch instance variables causes intermittent serialization failures

### 2.2 Governor Limit Sharing Between Package and Subscriber Org

Governor limits are **shared per transaction**, not per namespace. In a single transaction:
- **SOQL queries**: 100 total (shared across all namespaces)
- **DML statements**: 150 total (shared)
- **CPU time**: 10,000 ms total (shared)
- **Heap size**: 6 MB synchronous / 12 MB async (shared)

**Critical implication**: A managed package cannot safely use limits aggressively. ISV best practice is to reserve at least 50% of all limits for subscriber code. The package must be defensive about limit consumption.

**Limit monitoring strategies:**
```apex
// Track limits to leave headroom
Integer queriesUsed = Limits.getQueries();
Integer queriesLimit = Limits.getLimitQueries();
Integer remaining = queriesLimit - queriesUsed;
if (remaining < 10) {
    // Stop processing, defer to async
}
```

### 2.3 SOQL/DML Behavior Differences in Managed Contexts

- **`WITH SECURITY_ENFORCED`**: In managed packages, this enforces both namespace-level and org-level FLS. Fields in the package namespace are checked against the package's own FLS definitions
- **`WITH USER_MODE`** (API v56+, Winter '23): PMD rule `ApexCRUDViolation` does not flag queries using `WITH USER_MODE` since it respects FLS and object permissions automatically
- **Cross-namespace field references**: Must use fully qualified names `namespace__Field__c` in queries
- **Record ownership**: Records created by managed package code are owned by the running user, not the package
- **DML in constructors/initializers**: PMD rule `ApexCSRF` (PMD 5.5.3) flags this as error-prone. Salesforce raises a runtime exception for DML in constructors in managed packages

### 2.4 Caching Strategies for Managed Package Code

**Platform Cache:**
- Namespace-scoped by default. Managed package cache partitions are isolated
- Use `Cache.OrgPartition` for org-wide cache accessible across sessions and namespaces
- Use `Cache.SessionPartition` for per-user-session caching

**Static variables:**
- Isolated per namespace; subscriber code cannot access package static variables
- Should be used cautiously in managed packages since transaction scope is per-namespace for statics
- Static variables reset between transactions but persist for the lifecycle of a single Apex transaction

**Best practices:**
- Cache expensive describe results (using `SObjectDescribeOptions.DEFERRED` for lazy loading)
- Cache licensed feature configurations
- Cache metadata maps built from Custom Metadata Types
- Use Org Cache for data that changes infrequently (feature flags, license tiers)

### 2.5 Transaction Scope and Managed Package Boundaries

- A single transaction may include code from the subscriber org + multiple managed packages
- **Trigger execution order**: In a single DML operation, triggers execute in package install order. A record update can fire triggers from subscriber org + Package A + Package B
- **Transaction finalizers**: The `Finalizer` interface in managed packages can have its `execute()` method called even for unhandled exceptions in other namespaces
- **Savepoint/Rollback**: `System.Savepoint` and `Database.rollback()` affect all namespaces in a transaction. A managed package cannot roll back only its own DML

### 2.6 Heap and CPU Time Allocation Across Namespaces

- **No per-namespace allocation**: CPU time is a single pool for the entire transaction
- Managed packages should self-limit using `Limits.getCpuTime()` and `Limits.getLimitCpuTime()`
- Heap is also shared. Large collections in managed package code directly reduce available heap for subscriber code
- **Recommendation**: ISV packages should use `Limits` class defensively to check remaining limits before expensive operations

### 2.7 Debug Logging Challenges in Managed Packages

- Subscriber cannot see managed package debug logs by default
- PMD rule `AvoidDebugStatements` (PMD 6.36.0) notes: "Debug statements contribute to longer transactions and consume Apex CPU time even when debug logs are not being captured"
- PMD rule `DebugsShouldUseLoggingLevel` (PMD 6.18.0) recommends using `System.debug(LoggingLevel.WARN, ...)` for cleaner logs
- Managed package developers should use the Apex Replay Debugger and Checkpoints instead of `System.debug` for production
- Logging frameworks (e.g., Nebula Logger, custom Log__c object) circumvent debug log visibility issues by writing to custom objects

---

## 3. Salesforce Performance Optimization Techniques

### 3.1 SOQL Optimization

**Selective Queries:**
- Always include a `WHERE` clause (PMD `AvoidNonRestrictiveQueries` rule, PMD 7.4.0: "unfiltered SOQL or SOSL queries can quickly cause governor limit exceptions")
- **Always** include `LIMIT` when querying without a selective filter
- Use indexed fields (Id, Name, OwnerId, CreatedDate, SystemModStamp, RecordTypeId, lookup/master-detail fields, external ID fields, unique fields)

**Indexing:**
- Primary key (Id) is always indexed
- Custom indexes can be requested via Salesforce Support for fields with high selectivity
- Two-field indexes (composite) are automatically created for master-detail relationships

**Skinny Tables:**
- Salesforce-created copies of tables with only frequently-queried fields
- Maintained synchronously by the platform
- Must be requested via Salesforce Support
- No additional governor limit cost for using skinny tables
- Transparent to Apex code - the query optimizer automatically uses them

**Query Plan:**
- Use `Query Plan` tool in Developer Console to see whether a query uses indexes or full table scans
- Avoid leading `%` in `LIKE` clauses (prevents index use)
- Avoid negative filter operators (`!=`, `NOT`, `NOT LIKE`) - prevent index use
- Avoid formula field filters in WHERE clauses - not indexed

### 3.2 DML Bulkification Patterns

**PMD `OperationWithLimitsInLoop`** (PMD 6.29.0, Priority Medium 3):
Flags: Database class methods, DML operations, SOQL queries, SOSL queries, Approval class methods, Email sending, async scheduling or queueing within loops

```apex
// BAD - PMD violation
for (Account a : accounts) {
    insert a;
}

// GOOD - bulkified
insert accounts;
```

```apex
// BAD - PMD violation
for (Account a : accounts) {
    List<Contact> contacts = [SELECT Id FROM Contact WHERE AccountId = :a.Id];
}

// GOOD - collect IDs, query once
Set<Id> accountIds = (new Map<Id, Account>(accounts)).keySet();
List<Contact> contacts = [SELECT Id FROM Contact WHERE AccountId IN :accountIds];
```

**Partial success handling:**
```apex
Database.SaveResult[] results = Database.insert(records, false); // allOrNone=false
for (Database.SaveResult sr : results) {
    if (!sr.isSuccess()) {
        for (Database.Error err : sr.getErrors()) {
            // Handle individual errors
        }
    }
}
```

### 3.3 Apex Performance Patterns

**Collections and Maps:**
- Use `Map<Id, SObject>` for O(1) lookups instead of nested loops O(n^2)
- Prefer `Set<Id>` for membership checks
- Avoid `List.contains()` on large lists - use `Set.contains()` (O(n) vs O(1))

**Avoiding recursion:**
- Use static boolean flags to prevent trigger re-entry:
```apex
public class TriggerHelper {
    private static Boolean isExecuting = false;
    public static Boolean isExecuting() { return isExecuting; }
    public static void setExecuting(Boolean value) { isExecuting = value; }
}
```

**PMD rules for complexity:**
- `CognitiveComplexity` (PMD 6.22.0): Reports methods with complexity >= 15 (default). "Methods that are highly complex are difficult to read and more costly to maintain."
- `CyclomaticComplexity` (PMD 6.0.0): Reports methods with complexity >= 10 by default
- `AvoidDeeplyNestedIfStmts` (PMD 5.5.0, default depth 3): "Avoid creating deeply nested if-then statements since they are harder to read and error-prone to maintain."
- `TooManyFields` (PMD 5.5.0, max 15): "Classes that have too many fields can become unwieldy"
- `ExcessivePublicCount` (PMD 5.5.0, threshold 20): Classes with large numbers of public members increase testing complexity
- `NCSS Count` (PMD 7.19.0): Tracks Non-Commenting Source Statements - methods > 40 lines, classes > 500

### 3.4 Trigger Frameworks and Best Practices

**PMD `AvoidLogicInTrigger`** (PMD 5.5.0):
"Delegate the trigger's work to a regular class (often called Trigger handler class)."

**One Trigger Per Object pattern:**
```apex
trigger AccountTrigger on Account (
    before insert, before update, before delete,
    after insert, after update, after delete, after undelete
) {
    AccountTriggerHandler.handle(Trigger.new, Trigger.old, Trigger.operationType);
}
```

**PMD `AvoidDirectAccessTriggerMap`** (PMD 6.0.0):
"Avoid directly accessing Trigger.old and Trigger.new as it can lead to a bug. Triggers should be bulkified and iterate through the map."
```apex
// BAD
Account a = Trigger.new[0];

// GOOD
for (Account a : Trigger.new) { ... }
```

**Handler pattern best practices:**
- Use a virtual/dispatch pattern for flexible handler registration
- Control execution order with explicit handler ordering
- Enable/disable handlers via Custom Metadata or Custom Settings
- Bypass automation in tests via static flags

### 3.5 Async Processing Patterns

**PMD `AvoidFutureAnnotation`** (PMD 7.19.0, Priority Medium Low 4):
"Usage of the `@Future` annotation should be limited. Consider implementing the `Queueable` interface instead, which provides: Better error handling and monitoring capabilities, Support for more complex data types, Ability to chain jobs."

```apex
// NOT RECOMMENDED: @Future
@Future
public static void futureMethod(String accountId) { ... }

// RECOMMENDED: Queueable with Finalizer
public class QueueableExample implements Queueable, Finalizer {
    public void execute(QueueableContext context) {
        System.attachFinalizer(this);
    }
    public void execute(FinalizerContext ctx) { ... }
}
```

**PMD `QueueableWithoutFinalizer`** (PMD 7.8.0, Priority Low 5):
Detects when Queueable interface is used but a Finalizer is not attached. "Without attaching a Finalizer, there is no way of designing error recovery actions."

**Batch Apex considerations:**
- Use `Database.QueryLocator` (not iterable) for large data volumes (> 50,000 records)
- `Database.Stateful` interface for maintaining state across batch chunks
- PMD `AvoidStatefulDatabaseResult` (PMD 7.11.0): Avoid storing `Database.SaveResult` etc. in stateful batch instance variables (serialization failures)

### 3.6 Platform Cache and Org Cache Strategies

**Platform Cache types:**
- `Cache.OrgPartition` - Shared across all users in the org
- `Cache.SessionPartition` - Per-user session scope (8 hour TTL max)

**Cache best practices:**
- Cache expensive Schema describe results
- Cache Custom Metadata Type queries
- Cache feature flag configurations
- Use `Cache.CacheBuilder` interface for miss handlers:
```apex
public class MyCache implements Cache.CacheBuilder {
    public Object doLoad(String key) {
        // Load from database on cache miss
        return [SELECT ... FROM ... WHERE ...];
    }
}
```

**Note**: Platform Cache is not available in sandbox orgs by default; must be provisioned. Managed package cache partitions are namespace-scoped.

### 3.7 Event-Driven Architecture

- **Platform Events**: Decouple synchronous processing. Publish events for async handling. Reduced synchronous transaction time = less CPU usage, fewer limit violations
- **Change Data Capture (CDC)**: Efficiently track record changes without polling or triggers. Lower overhead than custom trigger-based change tracking
- **Streaming API (PushTopic, Generic)**: Real-time notifications without polling
- **Event Bus**: Managed package events are namespace-scoped. Subscribers can subscribe to package events if `global`

### 3.8 LWC/Lightning Performance Patterns

- **Wire adapters**: Use `@wire` to cache and deduplicate server calls. Avoid calling Apex imperatively when wire adapters can serve the same data
- **Lazy loading**: Load data on demand rather than all at initialization
- **Conditional rendering**: Use `if:true|false` to avoid rendering hidden components
- **Memoization**: Cache expensive computations in JavaScript properties
- **Avoid excessive Apex calls**: Batch related data retrieval into single Apex calls rather than multiple fine-grained calls

---

## 4. Existing Scanners and Static Code Analyzers

### 4.1 PMD / ApexPMD

**What it is**: Open-source static analysis tool maintained by the PMD project. Has first-class Apex language support with 7 rule categories (PMD 7.25.0, released 29-May-2026):

| Category | Rule Count | Key Rules |
|----------|-----------|-----------|
| **Performance** | 5 | `AvoidDebugStatements`, `AvoidNonRestrictiveQueries`, `EagerlyLoadedDescribeSObjectResult`, `OperationWithHighCostInLoop`, `OperationWithLimitsInLoop` |
| **Best Practices** | 9 | `AvoidGlobalModifier`, `AvoidLogicInTrigger`, `AvoidFutureAnnotation`, `QueueableWithoutFinalizer`, `ApexUnitTestClassShouldHaveAsserts`, `DebugsShouldUseLoggingLevel`, `UnusedLocalVariable` |
| **Design** | 11+ | `CognitiveComplexity`, `CyclomaticComplexity`, `AvoidDeeplyNestedIfStmts`, `ExcessiveParameterList`, `ExcessivePublicCount`, `TooManyFields`, `NcssCount`, `UnusedMethod` |
| **Error Prone** | 14 | `AvoidDirectAccessTriggerMap`, `AvoidHardcodingId`, `EmptyCatchBlock`, `EmptyIfStmt`, `OverrideBothEqualsAndHashcode`, `AvoidStatefulDatabaseResult`, `ApexCSRF`, `TestMethodsMustBeInTestClasses` |
| **Security** | 9 | `ApexCRUDViolation`, `ApexSOQLInjection`, `ApexSharingViolations`, `ApexBadCrypto`, `ApexInsecureEndpoint`, `ApexOpenRedirect`, `ApexSuggestUsingNamedCred`, `ApexXSSFromEscapeFalse`, `ApexXSSFromURLParam`, `ApexDangerousMethods` |
| **Code Style** | ~10 | Code formatting conventions |
| **Documentation** | ~2 | `ApexDoc` documentation requirements |

**Critical PMD Apex Performance Rules (detailed):**

1. **`OperationWithLimitsInLoop`** (PMD 6.29.0, Priority Medium 3): Catches DML, SOQL, SOSL, Approval, Email, and async operations inside loops. Java-based rule (not XPath), which means it uses full AST analysis.

2. **`AvoidNonRestrictiveQueries`** (PMD 7.4.0, Priority Medium 3): Identifies SOQL/SOSL without WHERE or LIMIT clauses. "When working with very large amounts of data, unfiltered SOQL or SOSL queries can quickly cause governor limit exceptions."

3. **`EagerlyLoadedDescribeSObjectResult`** (PMD 6.40.0, Priority Medium 3): Flags `getDescribe()` called without `SObjectDescribeOptions.DEFERRED`. Has property `noDefault` to enforce explicit options (since `DEFAULT` behavior changes between API v43 and v44).

4. **`OperationWithHighCostInLoop`** (PMD 7.0.0, Priority Medium 3): Catches `Schema.getGlobalDescribe()` and `Schema.describeSObjects()` in loops. These are expensive metadata operations.

5. **`AvoidDebugStatements`** (PMD 6.36.0, Priority Medium 3): Flags `System.debug()`. "Debug statements contribute to longer transactions and consume Apex CPU time even when debug logs are not being captured."

### 4.2 Salesforce Code Analyzer (Code Analyzer 5.x)

**Current version**: 5.13.0 (released May 25, 2026)

**What it is**: Salesforce's official unified code analysis tool. Wraps PMD, ESLint, and other engines into a single CLI.

**Key features:**
- Unifies PMD, ESLint, RetireJS, and CPD (copy-paste detection)
- Required for AppExchange Security Review submission
- Integrates with CI/CD via official GitHub Action
- Supports custom rules
- VS Code extension available for inline violation display
- Analyzes Apex, JavaScript, HTML, CSS, and Salesforce metadata (including Flows)

**Rules inherited from PMD**: Code Analyzer pulls PMD's full Apex rule set (all 7 categories above)

**Gaps**: Code Analyzer is primarily a **security review tool** with some performance rules. It does not have managed-package-specific rules. It does not check cross-namespace limit consumption or package-specific patterns.

**Repository**: `github.com/forcedotcom/code-analyzer` - TypeScript project, 237 stars, 52 forks

### 4.3 SonarQube/SonarCloud for Salesforce

- SonarSource provides Salesforce/Apex language support in both SonarQube (self-hosted) and SonarCloud (SaaS)
- Rules cover code smells, bugs, and security vulnerabilities specific to Apex
- Integration requires SonarQube Developer Edition or higher, plus the Apex plugin
- Provides Cognitive Complexity tracking (same metric as PMD `CognitiveComplexity` rule)
- Provides technical debt estimation
- Quality Gates for CI/CD integration

### 4.4 Commercial Tools

**Checkmarx:**
- Static Application Security Testing (SAST) focused
- Primarily security rules, limited performance rules
- AppExchange Security Review compatible

**Clayton:**
- Salesforce-specific code quality platform
- Automated code reviews
- Metadata analysis and governance

**CodeScan:**
- SonarQube-based cloud solution specifically for Salesforce
- Pre-built Salesforce-specific quality profiles
- Focus on security + best practices

**Gearset:**
- DevOps and deployment tool
- Includes static code analysis as part of CI/CD
- Comparison-based analysis (between environments)

### 4.5 Open Source Tools

**Apex-Link (Archived, moved to apex-ls):**
- "Salesforce metadata static analysis library focusing on Apex validation"
- Built in Scala, cross-compiled to JavaScript via Scala.js
- Used by: PMD, apex-assist VSCode extension, and various proprietary tools
- Provides: Full type resolution, method/field validation, "whole-program" analysis
- Supports multi-package analysis (1GP and 2GP)
- Was used to power PMD's `UnusedMethod` rule (now using apex-ls)

**Apex Language Server (apex-ls):**
- Successor to apex-link, maintained by apex-dev-tools organization
- v6.0.2 (Nov 2025)
- Provides: Code completion, error checking, type finding
- Powers PMD's rules that need type resolution (`UnusedMethod`, `AvoidInterfaceAsMapKey`)
- Configuration via `sfdx-project.json` under `plugins.apex-ls`:
  ```json
  {
    "plugins": {
      "apex-ls": {
        "dependencies": [{"namespace": "..."}],
        "additionalNamespaces": ["..."],
        "library": true,
        "maxDependencyCount": 10
      }
    }
  }
  ```
- Supports `ForceIgnoreVersion` (v1/v2), dependency management, external metadata
- Commands: `CheckForIssues` (with error/warning/unused detail levels), `DependencyReport`
- Output formats: text, json, pmd

### 4.6 What Gaps Exist in Current Tooling?

1. **No managed-package-specific performance rules**: None of the tools detect patterns unique to managed packages (cross-namespace limit exhaustion, excessive describe calls on subscriber schema, dynamic SOQL namespace injection overhead, license check overhead patterns)

2. **No governor limit cost estimation**: Static analyzers can't estimate how many SOQL queries, DML rows, or CPU ms a code path will consume

3. **No cross-namespace analysis**: PMD analyzes one codebase at a time. It cannot analyze how a managed package + subscriber code interact at the limits level

4. **No heap usage prediction**: Static analysis cannot predict heap consumption patterns

5. **No package-version-aware rules**: Rules don't consider whether code is in a beta, released, or patch version. `AvoidGlobalModifier` exists but doesn't consider packaging lifecycle (global is required for certain features)

6. **Limited trigger recursion detection**: Static analysis of trigger recursion (re-entry prevention) is surface-level at best

7. **No platform cache analysis**: No rules validate Platform Cache usage patterns, cache key design, or cache miss handling

8. **No LWC performance rules**: PMD/SonarQube focus on Apex backend. LWC performance (wire adapter usage, imperative call patterns, excessive @track usage, memory leaks) lacks static analysis coverage

9. **No comprehensive async-chain analysis in earlier tooling**: Basic queueable/batch/schedule chain patterns are now surfaced by Glade’s first-party performance plugin, but trace-backed cross-transaction verification is not yet available.

10. **No selector/domain layer enforcement**: No rules enforce SOQL-in-selector-layer patterns or DML-in-unit-of-work patterns (common ISV architecture patterns)

---

## 5. Managed Package Architecture Best Practices

### 5.1 Service Layer Patterns for ISV Apps

**Key principle**: Separate business logic from data access and presentation.

```
Aura/LWC Controller (presentation)
    └── Service Layer (business logic)
        ├── Selector Layer (SOQL queries)
        ├── Domain Layer (business rules, validation)
        └── Unit of Work (DML batching)
```

**Service Layer characteristics:**
- All `global` methods exposed to subscribers should be thin facades that delegate to `public` service methods
- Service methods should be stateless and bulkified
- Services should accept collections (not single records) wherever possible
- Services should implement `Limits` checking before expensive operations

### 5.2 Selector/Domain Patterns in Managed Context

**Selector Pattern:**
- All SOQL queries centralized in Selector classes
- Simplifies mocking in tests (query factory pattern)
- Enables consistent `WITH SECURITY_ENFORCED` / `WITH USER_MODE` application
- Centralizes field set management for subscriber-customizable query fields
- Entity-Relationship Diagram (ERD)-aware: automatically includes parent fields for relationship traversal

**Domain Pattern:**
- Business logic in domain classes, triggered by trigger handlers
- Domain classes implement validation, defaulting, complex calculations
- Decoupled from database access via Selector and Unit of Work layers

**Namespace handling in Selectors:**
```apex
public class AccountSelector {
    private static String NAMESPACE = MyApp__Constants.NAMESPACE;

    public static List<Account> selectById(Set<Id> accountIds) {
        return Database.query(
            'SELECT Id, ' + NAMESPACE + 'CustomField__c ' +
            'FROM Account WHERE Id IN :accountIds'
        );
    }
}
```

### 5.3 Unit of Work and DML Management

**Unit of Work (UOW) pattern:**
- Collects all DML operations throughout a transaction
- Executes them in a single, ordered DML execution step
- Ensures bulkification (no single-record DML)
- Handles partial success (allOrNone=false)
- Respects DML ordering (insert before relate, etc.)

**Key implementation:**
```apex
public class UnitOfWork {
    private List<SObject> insertList = new List<SObject>();
    private List<SObject> updateList = new List<SObject>();
    private List<SObject> deleteList = new List<SObject>();
    // ... registration methods ...

    public void commitWork() {
        if (!insertList.isEmpty()) Database.insert(insertList, false);
        if (!updateList.isEmpty()) Database.update(updateList, false);
        if (!deleteList.isEmpty()) Database.delete(deleteList, false);
    }
}
```

### 5.4 Dependency Injection and Mocking for Testability

**ISV testing challenge**: Managed packages must test with subscriber org data that may or may not exist.

**Patterns:**
- **Factory pattern**: Inject Selector/Service implementations via factory methods
- **`@TestVisible` annotation**: Expose private members for test access within the package namespace
- **`Test.isRunningTest()`**: Branch behavior for test scenarios (use sparingly)
- **Mock framework**: Use `System.StubProvider` for mocking interfaces
- **Selector mocking**: Override selector classes to return known test data

### 5.5 Feature Flags and Licensing Frameworks

**Common ISV patterns:**
- Custom Metadata Types for feature flags (cacheable, deployable)
- License tiers controlling feature access
- Per-transaction license checks add overhead - cache the result:

```apex
public class FeatureFlagService {
    private static Map<String, Boolean> featureCache;

    public static Boolean isEnabled(String featureName) {
        if (featureCache == null) {
            featureCache = new Map<String, Boolean>();
            for (FeatureFlag__mdt flag : FeatureFlagDAO.getAllActive()) {
                featureCache.put(flag.DeveloperName, flag.IsEnabled__c);
            }
        }
        return featureCache.containsKey(featureName)
            ? featureCache.get(featureName)
            : false;
    }
}
```

### 5.6 Error Handling and Logging Frameworks

**ISV error handling patterns:**
- Custom exception hierarchy (extend `Exception`)
- Managed package must gracefully handle subscriber org errors
- Never expose internal details in error messages to subscribers
- Use custom logging objects (e.g., `Log__c`) instead of `System.debug` for production error tracking
- PMD `EmptyCatchBlock` rule (PMD 6.0.0): Empty catch blocks swallow exceptions - "In most circumstances, this swallows an exception which should either be acted on or reported."
- PMD `ApexAssertionsShouldIncludeMessage` (PMD 6.13.0): Assertions should include descriptive messages for debugging

### 5.7 Version Compatibility and API Versioning

- Each Apex class in a managed package has an API version (set in Metadata XML or sfdx-project.json)
- Different classes can compile against different API versions
- Mixing API versions within a single package can cause behavioral inconsistencies
- Best practice: Set a consistent API version across the package, update with each major release
- `SObjectDescribeOptions.DEFAULT` behavior changes between API v43 and v44 (PMD `EagerlyLoadedDescribeSObjectResult` rule)

### 5.8 Namespace-Safe Dynamic SOQL/SOSL

**Pattern for namespace-agnostic dynamic queries:**
```apex
public class QueryBuilder {
    public static String qualifyField(String objectName, String fieldName) {
        String namespace = Schema.getNamespace();
        String prefix = String.isNotBlank(namespace) ? namespace + '__' : '';
        return 'SELECT ' + prefix + fieldName + ' FROM ' + objectName;
    }
}
```

Alternatively, use `String.escapeSingleQuotes()` for SOQL injection prevention (also flagged by PMD `ApexSOQLInjection` rule).

---

## 6. Testing and Monitoring

### 6.1 Performance Testing Approaches for Managed Packages

**Apex-level performance testing:**
- Use `Limits.getXxx()` to assert limit consumption in tests:
```apex
@IsTest
static void testPerformanceBudget() {
    Integer startQueries = Limits.getQueries();

    MyService.process(trigger.new);

    Integer queriesUsed = Limits.getQueries() - startQueries;
    System.assert(queriesUsed <= 10, 'Expected <= 10 queries, used: ' + queriesUsed);
}
```

**Load testing:**
- Test with maximum data volumes (bulk DML of 200 records)
- Test with large org data volumes (millions of records to test query selectivity)
- Test trigger recursion scenarios (parent-child updates)
- Test concurrent transaction contention

**PMD test quality rules:**
- `ApexUnitTestClassShouldHaveAsserts` (PMD 5.5.1): Requires at least one assertion per test class
- `ApexUnitTestClassShouldHaveRunAs` (PMD 6.51.0): Requires `System.runAs()` to test with different user contexts
- `ApexUnitTestShouldNotUseSeeAllDataTrue` (PMD 5.5.1): "Apex unit tests should not use @isTest(seeAllData=true) because it opens up the existing database data for unexpected modification by tests"
- `ApexUnitTestMethodShouldHaveIsTestAnnotation` (PMD 6.13.0): Enforces `@isTest` annotation over deprecated `testMethod` keyword

### 6.2 Limits Monitoring

**Event Monitoring (Real-Time Event Monitoring):**
- `ApexExecution` event type: Tracks CPU time, DML rows, SOQL queries per execution
- `ApexUnexpectedException` event type: Captures unhandled exceptions
- `ApexRestApi` event type: REST API limit consumption
- Available with Salesforce Shield or Event Monitoring add-on

**REST API Limits Resource:**
- `GET /services/data/vXX.X/limits` - Returns current and max limit values for all governor limits
- Can be called from Apex in a separate transaction to monitor org-wide limits
- Useful for ISV packages to check remaining limits before batch operations

**Limit Methods in Apex:**
```apex
// All available limit tracking methods
Limits.getAggregateQueries();
Limits.getAsyncCalls();
Limits.getCallouts();
Limits.getCpuTime();
Limits.getDMLRows();
Limits.getDMLStatements();
Limits.getEmailInvocations();
Limits.getFutureCalls();
Limits.getHeapSize();
Limits.getLimitXxx();  // corresponding getLimit methods
Limits.getPublishImmediateDML();
Limits.getQueueableJobs();
Limits.getQueries();
Limits.getQueryLocatorRows();
Limits.getQueryRows();
Limits.getSoslQueries();
```

### 6.3 Salesforce Optimizer Tool

- Available in Setup > Optimizer
- Analyzes org for:
  - Field usage and data skew
  - Apex performance anti-patterns
  - Storage usage
  - Object and field limits
- Generates a PDF report with recommendations
- ISV can run on subscriber orgs to identify performance issues
- Limited to the org being analyzed; cannot analyze package internals

### 6.4 Debug Logs and Apex Profiling

**Debug Log levels for managed packages:**
- Subscriber cannot view managed package debug logs (code is hidden)
- ISV partners can use Partner Debug Logs (special permission) for investigating subscriber issues
- In development: Set log levels per namespace for targeted debugging

**Apex Profiling:**
- Profiling information available in debug logs (Event Type = `PROFILING` or `SYSTEM_METHOD_ENTRY`/`SYSTEM_METHOD_EXIT`)
- Shows method-level CPU time breakdown
- Use Developer Console's "Analysis" perspective or log file tools

**Apex Replay Debugger:**
- VS Code integration for debugging Apex from debug logs
- Checkpoints can be set to capture variable state
- More efficient than `System.debug()` statements (PMD recommends over debug statements)

### 6.5 Custom Performance Metrics Collection

**Pattern: Custom performance logging object:**
```apex
public class PerformanceLogger {
    public static void log(String context, Map<String, Long> metrics) {
        Performance_Metric__c pm = new Performance_Metric__c(
            Context__c = context,
            Queries_Used__c = Limits.getQueries(),
            DML_Rows__c = Limits.getDMLRows(),
            CPU_Time_ms__c = Limits.getCpuTime(),
            Heap_Size__c = Limits.getHeapSize(),
            Timestamp__c = DateTime.now()
        );
        // Insert asynchronously to avoid affecting current transaction limits
        System.enqueueJob(new PerformanceLogQueueable(pm));
    }
}
```

**Considerations:**
- DML for performance logging itself consumes limits - use async methods
- Aggregate periodically to avoid data volume issues
- Use Platform Events for real-time metrics (decoupled from sync transaction)
- Correlate with `RequestId` for end-to-end transaction tracing

---

## Summary: Key Gaps for a New Scanner

Based on the comprehensive review of existing tools, here are the **highest-value gap areas** that a new managed-package-specific performance scanner could address:

1. **Cross-namespace limit analysis** - No tool analyzes governor limit consumption across managed package + subscriber code boundaries
2. **Managed package lifecycle rules** - No tool checks for version-compatibility violations, deprecated API patterns across package versions
3. **ISV-specific anti-patterns** - Excessive `global` usage cost, license check overhead, dynamic SOQL namespace injection patterns
4. **Query selectivity analysis** - Static prediction of whether SOQL queries will use indexes or cause full table scans
5. **Heap allocation prediction** - Static analysis of collection sizes and memory allocation patterns
6. **Trigger execution order analysis** - Multi-namespace trigger execution order and cumulative limit impact
7. **Platform Cache pattern validation** - Cache key design, miss handling, TTL strategy
8. **LWC performance rules** - Wire adapter usage, imperative call patterns, excessive @track/@api usage
