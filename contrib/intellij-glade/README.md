# Glade Apex Debugger for IntelliJ

This plugin lets JetBrains IDE users run and debug local Apex through the `glade` CLI.

## Requirements

- IntelliJ Platform build 253 or newer.
- LSP4IJ 0.19.4, installed as a plugin dependency.
- A `glade` binary on `PATH`.
- A local SFDX-style project when debugging project classes.

## Build

```bash
./gradlew --no-daemon clean test buildPlugin
```

## Run In A Sandbox IDE

```bash
./gradlew --no-daemon runIde
```

## Execute Anonymous Apex

Open a `.cls` or `.trigger` file, select Apex source, then choose `Glade: Execute Anonymous Apex`.

The plugin runs:

```bash
glade exec --debug-log - <selected source>
```

## Debug Anonymous Apex

Open a `.cls` or `.trigger` file, select Apex source, then choose `Glade: Debug Anonymous Apex`.

The plugin creates a temporary LSP4IJ DAP configuration and starts:

```bash
glade dap --project <project root>
```

The launch request sends the selected source in the DAP `source` field.

## Notes

The plugin does not register an Apex parser or claim `.cls` and `.trigger` file types. It enables actions by file extension to avoid conflicting with existing JetBrains Apex plugins.
