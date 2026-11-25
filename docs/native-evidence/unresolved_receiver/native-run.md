# Unresolved receiver capture

Salesforce Developer Edition scratch org. Capture ran on 2026-11-25 from 06:21:59 to 06:24:02 UTC. The scratch org was requested with a 30-day duration. Deletion and the post-delete check succeeded.

The anonymous request API and the test-class API were each 62.0, 67.0, and 68.0. Salesforce reported the deployed test class versions. The nine check-only deploys used the same three versions in both the request and each class metadata file. Each check-only row records the first component diagnostic. Embedded newlines are represented as `\n`. Test method rows record the exception type and message or a passing outcome. No case wrote records.

- An absent static receiver fails as a variable reference. A declaration with the same absent receiver name fails as a type reference. The available receiver runs.
- A test that calls a class with either compilation error reports the dependent-class diagnostic. The first error on the root class remains visible.
- Named tests get the same null exception from an absent field token, reflective type, and nested map value. The test using present values passes.
