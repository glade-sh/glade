# Duplicate rule evaluation captures

Captured 2026-11-13 at API 62.0 and 67.0. Evidence lives under `internal/apextest/testdata/conformance/duplicate_rule_evaluation/`. Each area contains the captured `org.tsv` and `RUN-FACTS.txt`. The replay uses synthetic masked inputs and no private capture material. Helper renames distinguish successive input panels. Metadata inputs reconstruct the activation state described in RUN-FACTS; the custom exact overflow rule is activated for its own stage.

Anonymous replay and the project test runner execute the same expressions. Sharing also replays anonymous standard-user access. Setup users are created in a separate setup transaction; committed criteria grants are replayed as ContactShare Rule records. No criteria grant is created for a test transaction.

Committed-seed searches and sharing overflow are excluded because RUN-FACTS records non-deterministic native indexing. Their raw rows remain intact.

`name_residuals.tsv` records exact native and current observations in the separately owned name scorers. It has no tolerance: both columns are pinned, including member sets, order, confidences, Find/error agreement and save verdicts. Six standard Account overflow observations retain company-name grade differences; two directional surname holdout panels retain the two pre-existing grade differences each. These are compatibility residuals, not native matches. This change does not modify the name scorers.

Unproven paths stay at prior behavior: Person Account corroboration without the captured exact surname and email, other subtype combinations, sharing overflow, settled committed indexing, and uncaptured upsert insert shapes.

Duplicate scans match before resolving record access. The user and object-wide sharing decision are resolved once per matched scan object. Enforce and Bypass keep detection separate from returned members.

A project with no declared sharing model keeps the prior duplicate outcome. The public-default Lead capture in `duplicate_public_defaults` confirms duplicate rejection under `@IsTest runAs` at 62.0 and 67.0. A standalone Contact at default sharing remains a capture item; its prior duplicate outcome is preserved. Undeclared-shape SOQL visibility is a separate existing gap.

The captured hidden matching field behavior applies to an optional custom Contact field only when project metadata explicitly denies read access to that field for the principal. An effective assigned grant restores access. An administrator without an explicit denial keeps the prior save and duplicate-detection outcomes. The sharing replay supplies explicit profile denials for the captured hidden field; it does not infer a real org's grants from omitted project metadata.

Criteria sharing rules are not computed. A committed criteria grant can be replayed as a ContactShare row, while an undeployed or test-created criteria rule does not reveal records.
