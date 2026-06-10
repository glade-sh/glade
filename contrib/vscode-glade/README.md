# Glade Apex Debugger

This is the opt-in VS Code debugger adapter for local Apex debugging with
`glade dap`.

## Install

Package and install the extension without the Marketplace:

```bash
npm install
npm run package
glade editor install vscode --vsix dist/vscode-glade-0.0.1.vsix --force
```

Check the local editor setup:

```bash
glade editor doctor vscode
```

The extension requires a global `glade` command on `PATH`.

## Develop

Install and compile:

```bash
npm install
npm run compile
```

Open the repo root in VS Code and run **Launch Glade VS Code Extension**. In the
Extension Development Host, open a Salesforce project and run commands from the
Command Palette:

- `Glade: Execute Anonymous Apex`
- `Glade: Debug Anonymous Apex`

## Breakpoint Smoke Test

Open the repo root in VS Code and run **Launch Glade VS Code Extension**. In the
Extension Development Host:

1. Open `/Users/matt/Dev/glade/internal/debuglog/testdata/project`.
2. Open `force-app/main/default/classes/TestProcessor.cls`.
3. Set a breakpoint inside `run()`, for example on the `insert a;` line.
4. Select `TestProcessor.run();` in an editor or enter it when prompted.
5. Run `Glade: Debug Anonymous Apex`.

The adapter launches `glade dap`, sends the source breakpoint, and runs the
anonymous Apex locally.
