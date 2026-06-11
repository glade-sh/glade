# Manifest Reference

Every plugin executable must return a `glade.plugin.v1` manifest.

```bash
./glade-plugin-quality manifest --json
```

## Shape

```json
{
  "apiVersion": "glade.plugin.v1",
  "name": "quality",
  "version": "1.2.0",
  "summary": "Project quality checks.",
  "commands": [
    {
      "path": ["quality"],
      "summary": "Run project quality checks."
    }
  ],
  "minimumGladeVersion": "0.1.0",
  "source": "github.com/acme/glade-plugin-quality"
}
```

## Rules

`name` and `version` become filesystem path segments. Use simple path-safe
tokens. The manifest name must match the package segment of the canonical
marketplace coordinate. For `@acme/quality`, the manifest name is `quality`.

Command paths advertise user-facing command roots. Dispatch uses the first
segment. A plugin cannot claim a built-in Glade root such as `test`, `schema`,
`inspect`, `plugins`, or another product command.
