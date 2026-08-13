# Build Glade from source

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Advanced task guide</p>
  <p>Build an unreleased Glade binary for product development or a source-only platform.</p>
</div>

## Prerequisites

- Go 1.26 or newer
- Git
- a C compiler with CGO enabled

On macOS, install Xcode Command Line Tools. On Debian or Ubuntu, install
`build-essential`.

## Build and verify

```bash
git clone https://github.com/glade-sh/glade.git
cd glade
go build -o glade ./cmd/glade
./glade version
```

An unreleased source build may report a development version. It is not the
stable artifact described by the public release manifest.

## Run against a project

```bash
cd path/to/salesforce-dx-project
path/to/glade/glade init --project . --yes
path/to/glade/glade doctor --project .
path/to/glade/glade check --project .
```

Use the [contributor and maintainer docs](/maintainer/) for repository tests,
release proof, and support-generation work.
