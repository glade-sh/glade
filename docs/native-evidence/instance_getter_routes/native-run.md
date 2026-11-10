Salesforce Developer scratch org; capture date 2026-11-09.
Deployed the initial pair of classes at their declared API versions and ran named tests. The other-instance test expected a second getter visit and failed with the recorded actual zero. A separate pair of backing tests asserted zero visits and passed; unbounded helper recursion failed with the recorded limit exception.
