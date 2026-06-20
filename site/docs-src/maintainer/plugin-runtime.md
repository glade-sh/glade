# Plugin Runtime

Plugins are executable processes. Glade installs them, checks their manifests,
writes lock files, and invokes their commands.

Keep plugin authoring out of the user guide until the registry is ready for a
larger public surface. User docs should show first-party install and lock-file
flows. Maintainer docs can show the runtime contract.

## Runtime contract

Each plugin executable must answer:

```bash
plugin-name manifest --json
```

The manifest declares commands, arguments, environment needs, and package
metadata. Glade validates the manifest before it exposes commands.

## Local proof

Use a linked executable while developing a first-party plugin:

```bash
glade plugins link --exec ./glade-plugin-compat
glade plugins lock --include-linked
```

Use registry or archive installs for team and CI lock files.
