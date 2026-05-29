# Notices

This project vendors generated parser sources from:

- `aheber/tree-sitter-sfapex`
- Commit: `94f1e55064e1136738c30ea9a326e63406107a35`
- License: MIT

Runtime binding:

- `github.com/tree-sitter/go-tree-sitter v0.25.0`

The generated Apex parser requires CGO. With `CGO_ENABLED=0`, the package still builds but `Parser.ParseSource` returns a diagnostic with code `APEXPARSECGO`.
