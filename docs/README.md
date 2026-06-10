# Docs Start Here

This map keeps first-use docs in one place and leaves deep planning trails out
of the way.

## If You Want To Use Glade

1. Install and first run: [INSTALL.md](INSTALL.md)
2. Run Apex tests without an org: [LOCAL_TESTING.md](LOCAL_TESTING.md)
3. Test startup cache (when it is safe to trust): [TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md)
4. Day-to-day editor commands: [EDITOR.md](EDITOR.md)
5. Current support surface:
   - Site first layer: <https://glade.sh/docs/guide/support-map>
   - [COMPATIBILITY_DASHBOARD.md](COMPATIBILITY_DASHBOARD.md)
   - [KNOWN_GAPS.md](KNOWN_GAPS.md)
   - [STDLIB_COVERAGE.md](STDLIB_COVERAGE.md)

## If You Want To Ship Releases

1. Release policy and claims: [RELEASE_POLICY.md](RELEASE_POLICY.md)
2. Operator checklist: [DISTRIBUTION_WORKFLOW.md](DISTRIBUTION_WORKFLOW.md)
3. Ongoing notes: [RELEASE_NOTES.md](RELEASE_NOTES.md)

## If You Want To Contribute

1. Runtime architecture: [ARCHITECTURE.md](ARCHITECTURE.md)
2. Where to add Salesforce functionality: [ADDING_A_PLATFORM_API.md](ADDING_A_PLATFORM_API.md)
3. Surface Ledger local runbook: [ADDING_A_PLATFORM_API.md#finding-the-next-gap-instead-of-waiting-for-a-failure](ADDING_A_PLATFORM_API.md#finding-the-next-gap-instead-of-waiting-for-a-failure). Start with `compat surface sources`, then `compat surface refresh`, then `compat surface packet`.
4. Package boundaries and conventions: [ARCHITECTURE_STANDARDS.md](ARCHITECTURE_STANDARDS.md)
5. Storage model and DB behavior: [storage-schema.md](storage-schema.md)
6. Example-project harness: [EXAMPLE_PROJECTS.md](EXAMPLE_PROJECTS.md)
7. Apex declaration parser: [APEX_PARSER.md](APEX_PARSER.md)

## Planning And Backlog Docs

These are working design and backlog trails, not first-use docs:

- [LOCAL_APEX_TEST_EXECUTION_PLAN.md](LOCAL_APEX_TEST_EXECUTION_PLAN.md)
- [APEX_PARITY_FOLLOWUP_PLAN.md](APEX_PARITY_FOLLOWUP_PLAN.md)
- [POST_PARITY_TODO.md](POST_PARITY_TODO.md)
- [FEATURE_PARITY_TODO.md](FEATURE_PARITY_TODO.md)
- [MANAGED_PACKAGE_DEPENDENCY_PLAN.md](MANAGED_PACKAGE_DEPENDENCY_PLAN.md)
- [BEHAVIORAL_STUB_SUPPORT_PLAN.md](BEHAVIORAL_STUB_SUPPORT_PLAN.md)
