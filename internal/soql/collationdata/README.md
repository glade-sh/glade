# Generated collation tables

Generate these files with `python3 ../collation_generate.py` from this
folder, or `go generate` in the parent package. The generator requires public
ICU 78.1 APIs reporting CLDR 48. For the initial import, pass
`--cldr-archive` with the official CLDR 46 core.zip. Later generation reuses
the checked-in root input; no native observation is a generator input.

- `cldr_root.txt`: the CLDR 46 allkeys_CLDR.txt weight rows and directives,
  stored as UTF-8 text. Character-name comments are
  omitted; all weight rows are preserved. Original archive/source hashes and
  the generated payload hash are recorded in manifest.json. The public x/text root builder consumes this table.
- `elements_root.json`: scalar elements obtained from ICU's public
  collation-element iterator. It covers all Unicode scalar values, including
  implicit weights. Lossless arithmetic runs keep the table compact.
- Other `*.json`: changed scalar runs, canonical contractions and primary
  reorder units for one public locale tailoring. Locale aliases share an
  immutable table, loaded only on first use. No ICU runtime is required.
- `manifest.json`: generated locale/profile policy, default attributes,
  source versions, generator hash and payload hashes. Latvian retains the previous comparator because
  the supplied native rows differ from the newer public tailoring.

Every payload is plain UTF-8 text; table payloads are JSON with format GLDC2.
The existing leak scanner inspects every generated string without decompression. Scalar ranges hold first/last scalar,
element values and signed strides. Contractions hold text and element lists;
reorder pairs map primary units. Every scalar is observed before a run extends.
An element uses the public iterator's 16-bit primary, 8-bit secondary and
8-bit tertiary layout. Continuation elements carry trailing primary units.
Case bits are retained for upper-first and lower-first defaults; secondary
comparisons discard tertiary differences. Shifted variables and their
following primary ignorables are omitted through tertiary strength.

Primary reorder data is sampled from public sort keys at 16-bit granularity;
a lead byte alone can contain multiple scripts. The exporter visits every
assigned non-Han scalar plus every tailored scalar and root primary unit.
Its manifest records conflicts; generated profiles currently have none.

The x/text builder treats single primaries at FFFD as an incomplete implicit
weight. The root loader uses its public weight-renumbering contract to encode
that last regular primary with the highest-weight escape. CLDR's FFFE has
primary 1 and remains separate. This does not discard the replacement row.

The runtime caches only immutable generated data. Each query owns its
collator buffers and string-key memoization. Equality uses the existing
field equality path and is not redirected through these tables.

Unicode and upstream notices are in `third_party/unicode-collation` and are
included by the release notice generator.

Run `python3 ../collation_generate.py --check` from this folder to regenerate
all tables in a temporary directory and compare every byte, including the
manifest, without editing the checkout. `--output-dir DIR` writes a separate
export. Both commands use the checked-in CLDR root input by default.
`TestLocaleCollationGeneratedDataIntegrity` detects drift in any payload or the
generator against the manifest on every platform, without an ICU dependency.
Regeneration requires exactly ICU 78.1 / CLDR 48, checked before exporting.
