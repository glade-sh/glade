<p align="center">
  <img src="site/docs-src/public/logo-mark.svg" alt="Glade boxed contour mark" width="96" height="96">
</p>

<h1 align="center">Glade</h1>

<p align="center">Local Apex runtime and developer tools for Salesforce projects.</p>
<p align="center">
  <a href="https://glade.sh">Site</a> ·
  <a href="https://glade.sh/guide/quickstart">Quickstart</a> ·
  <a href="https://glade.sh/guide/support-map">Support map</a> ·
  <a href="https://glade.sh/guide/security-trust">Security</a> ·
  <a href="https://github.com/glade-sh/glade/issues/new/choose">Feedback</a>
</p>
<p align="center">
  <a href="https://github.com/glade-sh/glade/actions/workflows/security.yml"><img alt="Security workflow" src="https://github.com/glade-sh/glade/actions/workflows/security.yml/badge.svg?branch=main"></a>
  <a href="https://scorecard.dev/viewer/?uri=github.com/glade-sh/glade"><img alt="OpenSSF Scorecard" src="https://api.scorecard.dev/projects/github.com/glade-sh/glade/badge"></a>
</p>

Glade is an open-source, clean-room local Apex runtime and developer toolkit.
Run supported tests, inspect and debug code, and exercise local SOQL, DML, and
triggers without deploying to an org. Keep Salesforce for final validation
and hosted platform behavior.

Glade reads source and metadata from disk and executes supported behavior in
its own runtime. The CLI, VS Code extension, and local browser tools are
interfaces to that runtime—not a hidden Salesforce org.

Glade is an independent open-source project and is not affiliated with,
sponsored by, or endorsed by Salesforce. Salesforce and Apex are trademarks of
Salesforce, Inc.

## Get your first result

```bash
curl -fsSL https://glade.sh/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
glade version
```

Continue with the [canonical quickstart](https://glade.sh/guide/quickstart):

- **Try the sample:** load the built-in Refinement Service, then run its named test.
- **Use my project:** initialize an existing Salesforce DX project and run one known test class before the full suite.

The corrected sample and named test ship in the **v0.2.15 stable release**. A
playground Pass alongside source errors is not valid proof; the quickstart shows
the named, nonzero test result to expect.

For a project with `RefinementServiceTest` (substitute your actual class):

```bash
test -f glade.yml || glade init --project . --yes
glade doctor --project .
glade check --project .
glade test --project . --class RefinementServiceTest --json --no-progress
```

A first test result must name at least one executed test. Zero tests is not a
passing evaluation. The [installation guide](docs/INSTALL.md) covers macOS/Linux
archives, pinning, and verification; source development has a
[separate guide](https://glade.sh/guide/build-from-source).

## Three useful loops

| Job | Local result | Next step |
| --- | --- | --- |
| Develop and debug | Run one test, inspect a failure, set a breakpoint, fix and rerun | [Tests and debugging](https://glade.sh/guide/editor) |
| Work with data | Seed isolated local state and exercise SOQL, DML, and triggers | [Local data](https://glade.sh/guide/workflows/local-data) |
| Automate feedback | Use the same CLI in an editor, advisory CI, or an AI-assisted workflow | [CI](https://glade.sh/guide/ci-artifacts), [AI workflow](https://glade.sh/guide/ai-assisted-apex) |

LWC/Visualforce previews, local HTTP APIs, package contracts, and report tools
are deeper workflows. Start with [How Glade works](https://glade.sh/guide/modules)
or [Choose a workflow](https://glade.sh/guide/workflows).

## Know the boundary

- Supported local behavior is not blanket Salesforce parity. Hosted services,
  live auth, deployment, exact Lightning Experience behavior, and final
  production validation remain with Salesforce.
- Checked Apex source versions are 65.0, 66.0, and 67.0. Historical sources may
  be preserved without checked correctness credit; Execute Anonymous and LWC
  have stricter eligibility. Checked HTTP endpoints are a separate axis:
  60.0, 65.0, 66.0, and 67.0.
- Local test isolation is not an OS sandbox. Plugins run as your user.
  Review network use by custom code and external AI providers.
- Preview and deterministic harness behavior have named limits. A supported
  harness row does not implement the corresponding live hosted service.

Use the [support map](https://glade.sh/guide/support-map),
[known gaps](docs/KNOWN_GAPS.md), [stdlib ledger](docs/STDLIB_COVERAGE.md),
[compatibility policy](docs/COMPATIBILITY.md), and
[Apex language contract](docs/APEX_LANGUAGE_COMPATIBILITY.md) for detail.
Counts apply to their named catalogs, not all Salesforce behavior.

## Optional first-party plugins

[Glade Tools](https://github.com/glade-sh/glade-tools) contains maintenance and
extension workflows. `@glade/performance` provides advisory scans;
`@glade/orgpackage` supports package contracts; `@glade/compat` is
maintainer-facing compatibility tooling. Product and plugin versions are
independent—verify the pairing your team uses.

See [plugin installation and trust](docs/PLUGINS.md). Glade Tools source and
first-party plugin archives are licensed under Apache-2.0 and carry project and
bundled dependency notices. Base Glade does not depend on Tools internals.

## Feedback and contributions

[Report a bug or tell us about your workflow](https://github.com/glade-sh/glade/issues/new/choose).
Include version, OS/architecture, command, expected/actual result, test count,
and a minimal public reproduction. Do not share proprietary source, private
package names, credentials, or customer records.

Use [private vulnerability reporting](https://github.com/glade-sh/glade/security/advisories/new)
for security issues. See [SECURITY.md](SECURITY.md) and
[CONTRIBUTING.md](CONTRIBUTING.md). The [pilot guide](https://glade.sh/guide/tester-field-guide)
helps teams evaluate one representative path before making it a merge gate.

Glade source, documentation, examples, site source, and the VS Code extension
are licensed under the [Apache License 2.0](LICENSE). Copyright 2026 Matt Simonis.

## References

- [CLI reference](https://glade.sh/reference/cli)
- [Project configuration](docs/CONFIG.md)
- [Local Apex testing](docs/LOCAL_TESTING.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Editor and debug setup](docs/EDITOR.md)
- [Local LWC shell](docs/LWC_LOCAL_SHELL.md)
- [Security and release trust](docs/SECURITY_TRUST.md)
- [Release policy](docs/RELEASE_POLICY.md)
- [Release notes](docs/RELEASE_NOTES.md)
