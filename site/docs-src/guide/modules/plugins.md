# Plugins

Plugins own extension workflows that stay outside the base `glade` product.
They install, link, list, and run standalone plugin executables. They do not
belong inside the base runtime.

## Use it when

Use plugins when a workflow belongs outside the base `glade` product or must
run through a standalone extension executable.

## Entry commands

```bash
glade plugins list
glade plugins available
glade plugins link --exec <plugin-executable>
```

## What this module owns

- Installed, linked, and available plugin discovery.
- Plugin execution, lock files, and CI restoration for extension workflows.
- A clean boundary for first-party maintenance and advisory tools outside the
  base runtime.

## Requires Salesforce

Salesforce remains the validation gate for hosted platform behavior.

Some plugins may capture org facts or run advisory checks against Salesforce.
The base product must not depend on plugin internals.

## Related workflows

- [Plugins guide](/guide/plugins)
- [First-party plugins](/guide/plugins/first-party)

## Reference

- [Plugin lock files and CI](/guide/plugins/lock-ci)
