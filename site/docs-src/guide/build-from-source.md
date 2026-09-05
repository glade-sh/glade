---
pageType: guide
canonicalTask: /guide/build-from-source
---

# Build Glade from source

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Advanced task guide</p>
  <p>Build an unreleased Glade binary for product development or a source-only platform.</p>
</div>

## Prerequisites

- The Go version required by the checkout’s `go.mod`
- Git
- a C compiler with CGO enabled

On macOS, install Xcode Command Line Tools. On Debian or Ubuntu, install
`build-essential`.

## Build and verify

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
CGO_ENABLED=1 go build -o bin/glade ./cmd/glade
./bin/glade version
./bin/glade doctor --json
```

Check `parserOK` in doctor’s JSON output. Doctor also checks project setup;
other findings depend on the project selected. A successful build or version
command alone does not verify parser availability.

An unreleased source build may report a development version. It is not the
stable artifact described by the public release manifest.

## Run against a project

```bash
cd path/to/salesforce-dx-project
test -f glade.yml || /absolute/path/to/glade/bin/glade init --project . --yes
/absolute/path/to/glade/bin/glade doctor --project .
/absolute/path/to/glade/bin/glade check --project .
```

Use the [contributor and maintainer docs](/maintainer/) for repository tests,
release proof, and support-generation work.
