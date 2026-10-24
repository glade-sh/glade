# Workflow validation

CI's `workflow-lint` job installs actionlint **1.7.12** from the upstream
[release](https://github.com/rhysd/actionlint/releases/tag/v1.7.12).
`scripts/install-actionlint.sh` pins the Linux amd64 archive SHA-256 to
`8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8`.
It verifies the downloaded archive before extraction or execution. The CI job's
checkout and Go setup actions use full commit pins with version comments.

With actionlint on PATH, run from the repository root:

```sh
scripts/ci-actionlint.sh
scripts/ci-go-test.sh lane workflow-lint
```

The wrapper checks every `.github/workflows/*.yml` and `*.yaml`, including when
called from another directory. Explicit file arguments are resolved relative to
the caller. It fails if actionlint is missing or discovery finds no workflows.
Tests skip real-actionlint cases with an explicit message only when the tool is
absent; CI always installs it first. Tests never download anything. The corrupt
archive test stubs curl and verifies that extraction cannot run on bad bytes.
Optional ShellCheck and Pyflakes integrations are disabled to keep this check
focused on workflow syntax, expressions, contexts and action configuration.
No actionlint errors are ignored.

The CI job uses the lane and resource wrappers, uploads raw test events and
resource usage even on failure, and is a dependency of `Required CI`. The lane
runs only the offline actionlint tests, with a three-minute timeout. Its raw
events are written to `ci-artifacts/go-test/test-workflow-lint.json`.

For a local macOS install, download the matching archive from that release and
verify its digest against the upstream `actionlint_1.7.12_checksums.txt` before
extracting. The Darwin arm64 archive SHA-256 is
`aba9ced2dee8d27fecca3dc7feb1a7f9a52caefa1eb46f3271ea66b6e0e6953f`.

To update the pin, choose an exact upstream release, review its release notes,
download the Linux amd64 archive and published checksums, and independently
compute the archive's SHA-256. Update `version` and `sha256` together in the
installer and update this document. Repeat the commands above with the new
binary; the job-env fixture must be rejected and the step-env fixture accepted.
Review any real-workflow errors before committing the pin change. Keep action
references pinned to full commits with version comments when updating the job.
