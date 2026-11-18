# Unicode collation data

The UTF-8 collation weights and tailorings in
`internal/soql/collationdata` derive from Unicode CLDR 46 root data and
CLDR 48 data exposed by the public ICU 78.1 collation APIs.

Copyright © 1991 and later: Unicode, Inc. and others.

LICENSE contains the canonical CLDR 46 Unicode License V3.
LICENSE-ICU contains the canonical Unicode License V3 copyright and permission
notice for ICU 78.1. These are data exports; no ICU implementation or its
third-party software components are incorporated into Glade. The exporter
and runtime table reader are Glade code. The release notice generator includes
these files in THIRD_PARTY_NOTICES/data/unicode-collation, alongside the
licenses copied for linked Go modules.

Sources:
- https://www.unicode.org/Public/cldr/46/core.zip
- https://github.com/unicode-org/icu/blob/release-78.1/LICENSE
- https://unicode-org.github.io/icu/userguide/collation/api.html

Run `go generate` in `internal/soql` with the pinned ICU generation library.
See `internal/soql/collationdata/README.md` for the format and provenance.
