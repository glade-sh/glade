# Restricted picklist null ordering

Rows 2–13 of `restricted_picklist_null_order.tsv` record anonymous Apex observations at source and transport API 62.0 and 67.0. The 62.0 capture was repeated with the same result. The owned custom picklist was restricted, declared in the order Zulu, Alpha, Middle, and held those three values plus a null. `v0`, `v1`, `v2`, and `v3` denote those values in that order. Each query ordered by the picklist followed by `Id ASC`.

The 62.0 rows show that explicit NULLS clauses are ignored for this shape; the 67.0 rows show that they are honored. This does not establish behavior for unlisted values, missing metadata, or source versions other than 62.0 and 67.0.
