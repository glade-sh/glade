# LWC Local Support

This page lists the user-facing LWC support surface for the local shell and
Visualforce Lightning Out host.

## Hosts

| Host | Status | Notes |
| --- | --- | --- |
| Direct component shell | Supported for local development | `/lwc/preview/component/<namespace>/<component>` mounts one exposed component. |
| Record page shell | Supported for local development | `/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>` resolves FlexiPage regions and record context. |
| App page shell | Supported for local development | `/lwc/preview/app/<Page>` resolves app-page FlexiPage metadata. |
| Home page shell | Supported for local development | `/lwc/preview/home/<Page>` resolves home-page FlexiPage metadata. |
| Custom tab shell | Supported with limits | LWC tabs and FlexiPage tabs render locally. Visualforce tabs redirect to `/apex/<Page>`. Web and object tabs are reported as unsupported LWC-shell targets. |
| Visualforce Lightning Out | Supported with limits | `/apex/<PageName>` can host LWCs through `$Lightning.use()` and `$Lightning.createComponent()` using the shared local runtime. |

## Runtime Services

| Service | Status | Host coverage |
| --- | --- | --- |
| LWC module compilation | Supported | LWC shell and Visualforce Lightning Out. |
| Import maps | Supported | LWC shell and Visualforce Lightning Out. |
| Apex wire and imperative Apex imports | Supported with VM limits | LWC shell and Visualforce Lightning Out. |
| `lightning/uiRecordApi` `getRecord` | Supported with local data limits | LWC shell and Visualforce Lightning Out. |
| `lightning/uiRecordApi` `getObjectInfo` | Supported with local schema limits | LWC shell and Visualforce Lightning Out. |
| `lightning/uiRecordApi` create, update, and delete helpers | Supported with local DML limits | LWC shell and Visualforce Lightning Out. Mutations use the local DML engine for supported objects, including ID sequences, required-field checks, audit fields, explicit nulls, and soft deletes. |
| `lightning/uiRecordApi` field helper functions | Supported for local record shapes | LWC shell and Visualforce Lightning Out. |
| `@salesforce/schema` object and field tokens | Supported | LWC shell and Visualforce Lightning Out. |
| `@salesforce/label` | Supported with local metadata limits | LWC shell and Visualforce Lightning Out. |
| `@salesforce/resourceUrl` | Supported with local metadata limits | LWC shell and Visualforce Lightning Out. |
| `@salesforce/contentAssetUrl` | Supported with local metadata limits | LWC shell and Visualforce Lightning Out. |
| `@salesforce/user` | Supported for `Id` and `isGuest` | LWC shell and Visualforce Lightning Out. |
| `@salesforce/i18n` | Supported for checked local values | LWC shell and Visualforce Lightning Out. |
| `lightning/navigation` | Supported with local route limits | LWC shell and Visualforce Lightning Out. |
| `lightning/messageService` | Supported for in-page publish and subscribe | LWC shell and Visualforce Lightning Out. |
| `lightning/platformResourceLoader` | Supported for local scripts and styles | LWC shell and Visualforce Lightning Out. |
| `lightning/platformShowToastEvent` | Supported as a browser event shim | LWC shell and Visualforce Lightning Out. |

## Unsupported Or Limited

| Area | Local behavior |
| --- | --- |
| Web custom tabs | Unsupported in the LWC shell. |
| Object custom tabs | Unsupported in the LWC shell. |
| Missing LWC, FlexiPage, or tab metadata | Returns a named `GLADELWC` diagnostic. |
| Full Lightning Experience | Not modeled. Use Salesforce for hosted chrome, app state, console APIs, live auth, permissions, and final gates. |
| Full UI API | The local shell has selected LDS/UI API shims. It is not broad UI API parity. |
| Full base component and SLDS parity | Use the supported modules in this build. Keep a Salesforce browser check for exact styling and hosted base-component behavior. |
| Exact Visualforce Lightning Out parity | The local host mounts LWCs and shares runtime services. Hosted lifecycle timing and every Lightning Out edge are not promised. |

## First Commands

```bash
glade toolchain install
glade dev lwc --project . --port 8080
```

Then open a route from the startup banner.
