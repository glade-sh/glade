# Phase 3 Base Component Source Matrix

This note records the source and dependency check for Capability Phase 3 base
components. Glade keeps these shims as Glade-owned runtime modules.

| Source | License check | Build fit | Decision |
| --- | --- | --- | --- |
| `lightning-base-components` | Registry reports MIT; the tarball includes Salesforce terms. | A narrow dependency install does not break Glade when unused. The full source tree depends on many Salesforce-private modules and dynamic component compiler flags. | Reference behavior and markup only. Do not add as a runtime dependency. |
| `@salesforce-ux/design-system` | BSD-3-Clause package. Some icons and images use separate CC BY-ND terms. | CSS, sprite, font, and image assets fit the local SLDS asset lane. No base-component behavior. | Vendored as the classic SLDS fallback asset tree with package notices. |
| `@salesforce-ux/design-system-2` | Includes Salesforce terms. | Useful shell and styling reference. No LWC behavior. | Vendored as the default SLDS 2 local stylesheet and asset tree with package notices. |
| `jerry-wang12/lightning-demo` | `package.json` says MIT; no repo-level license file was present in the probe. | Contains many Lightning component folders, but the build scripts are stale and do not run clean. | Reference only. Do not copy source without a separate provenance decision. |
| `@salesforce/design-system-react` | BSD-3-Clause. | React library, not LWC. | Do not use for runtime base components. |
| `@salesforce/lightning-types` | Salesforce terms. | Type declarations only. | Contract reference only. |

The local implementation covers practical DOM and common events for the Phase 3
set: email, dual listbox, select, slider, rich text input, menu divider,
progress bar/ring, tile, breadcrumbs, tree grid, map, carousel, quick action
panel, record picker, file upload, and the package-first display/input/container
components. Exact Salesforce base-component internals remain a hosted browser
check.
