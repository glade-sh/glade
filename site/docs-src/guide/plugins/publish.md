# Publish a plugin

Marketplace publication starts as a pull request to the curated catalog. The
catalog points to release archives at durable URLs.

```bash
glade plugins install @acme/quality@1.2.0
glade plugins info @acme/quality
```

## Publication flow

1. Build platform archives.
2. Publish archives at durable release URLs.
3. Submit a marketplace entry by pull request.
4. Let CI download every asset.
5. Let CI verify archive SHA-256.
6. Let CI verify archive layout.
7. Let CI verify `checksums.txt`.
8. Let CI run `manifest --json`.
9. Fix unsafe names, unsafe versions, missing assets, broken docs links, or
   command roots that collide with built-in Glade commands.
10. Approved entries merge into the curated catalog.

The registry contract does not depend on a hosted publisher portal. Custom
registries can use the same format before a plugin enters the default
marketplace.
