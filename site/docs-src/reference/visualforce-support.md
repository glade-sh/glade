---
pageType: reference
canonicalTask: /guide/workflows/visualforce-preview
---

# Visualforce support matrix

Inspect the running local renderer's support rows and its preview boundary.
The endpoint reports the binary you started, so it stays bound to that local
implementation.

Use this page when checking the local Visualforce preview boundary.
The support map carries the current local contract and hosted gaps.

## Start here

Start with controller logic, page routes, standard components, static resources, uploads, remoting, and AJAX refresh paths.
Then check the detailed source for current limits.

Glade serves Visualforce previews for local development.
It does not promise Salesforce chrome, exact lifecycle timing, or byte-for-byte PDF output.
Use Salesforce for hosted rendering proof.

## Inspect the running implementation

<!--@include: ../../../docs/LOCAL_TESTING.md#run-visualforce-pages-locally-->

The local support JSON is an implementation inventory. It is not a Salesforce
rendering comparison. Record the binary version and the tested page when
reporting a mismatch.

## Detailed source

[What Glade runs locally](/guide/support-map)

## Related workflows

- [Visualforce preview module](/guide/modules#visualforce-preview)
- [Visualforce preview workflow](/guide/workflows/visualforce-preview)
- [LWC preview module](/guide/modules#lwc-preview)
