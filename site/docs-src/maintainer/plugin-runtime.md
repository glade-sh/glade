# Plugin Runtime

Plugins are executable processes. Glade installs them, checks their manifests,
writes lock files, and invokes their commands.

Keep plugin authoring in the maintainer guide. User docs show first-party
install and lock-file flows; maintainer docs own the executable and archive
contracts.

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
