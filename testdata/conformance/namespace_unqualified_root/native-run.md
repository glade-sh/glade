Capture date: 2026-11-07 UTC. Owned synthetic sources only; fresh scratch orgs are Winter '27 (ruling 54). Locale and timezone unmeasured. Captured for the v0.3.5 namespace family (ruling 49), worker ns2.

Org: one fresh one-day Developer scratch org with no namespace, no features and no package, created from an authorized campaign Dev Hub. Existing scratch-HOME auth. No credentials, org identifiers, usernames or hub identities are recorded here.

Question: an unqualified root that names no class in the org and no namespace (RootV035; the org holds no class named RootV035 and no such namespace). What do `RootV035.Inner x;`, `new RootV035.Inner()`, a static field read `RootV035.Inner.VALUE`, a static method call and the other type positions give, at API 62.0 and 67.0, on the anonymous route and in @IsTest classes?

Anonymous route: every anon/*.apex file is an independent `sf apex run --api-version <api> --file <case>` at 62.0 and 67.0 (20 runs). Each file holds one rejection, because anonymous compilation reports only the first error. org.tsv records the compile outcome and the first diagnostic with line and column as reported.

@IsTest route: the eight named/*.cls @IsTest classes were deployed together in one check-only deploy per API (`sf project deploy start --source-dir force-app --dry-run --test-level NoTestRun --api-version <api>`, class metadata apiVersion matching). Each class isolates one position (field, parameter, return, local, new, cast, static field read, static method call). The deploy reported one component failure per class; zero tests ran and nothing was persisted.

Result: the root is indistinguishable from an absent namespace. Type positions (local, new, cast, generic argument, field, parameter, return) give `Invalid type: RootV035.Inner`; a static field read and a static method call give `Variable does not exist: RootV035.Inner` (the receiver, without the member); a three-part read `RootV035.Inner.Deep.VALUE` gives `Variable does not exist: RootV035.Inner.Deep`. Controls: `RootV035 x;` is `Invalid type: RootV035` and `RootV035.COUNT` is `Variable does not exist: RootV035`. All rows identical at 62.0 and 67.0 and across routes.

Deletion: `sf org delete scratch --no-prompt` returned status 0 for the capture alias; the Dev Hub ScratchOrgInfo row for OrgName "glade v035 ns2 root capture" reads Deleted (queried after the delete).
