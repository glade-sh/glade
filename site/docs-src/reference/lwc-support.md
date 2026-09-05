---
pageType: reference
canonicalTask: /guide/workflows/lwc-preview
---

# LWC support matrix

Compare local host and service behavior below. Tables are included directly
from the maintained repository LWC support source during the site build.

Use this page when checking whether a Lightning Web Component can run in the local shell.
The detailed guide and checked support rows carry the route and module detail.

## Start here

Start with the local LWC shell guide for how to launch and navigate previews.
Then use the support rows when you need module-level facts.

Glade models useful local LWC development paths.
It does not promise exact Lightning Experience chrome, hosted Flow Builder behavior, or every `lightning-*` edge.
Keep Salesforce validation for hosted platform behavior.

## Hosts

<!--@include: ../../../docs/LWC_SUPPORT.md#hosts-->

## Runtime services

<!--@include: ../../../docs/LWC_SUPPORT.md#runtime-services-->

## Limits and evidence

These are local implementation labels, not current Salesforce verification.
The source's `supported-local` capture status means local browser evidence;
`supported` rows require separate local and Salesforce comparison evidence.
A prepared target or download release label does not establish either result.

## Detailed source

- [Local LWC Shell](/guide/lwc-local-shell)
- [LWC support rows](https://github.com/glade-sh/glade/blob/main/docs/LWC_SUPPORT.md)

## Related workflows

- [LWC preview module](/guide/modules#lwc-preview)
- [LWC preview workflow](/guide/workflows/lwc-preview)
- [Visualforce preview module](/guide/modules#visualforce-preview)
