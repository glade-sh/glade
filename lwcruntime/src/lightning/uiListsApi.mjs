import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";

function objectConfig(config) {
  // Native null/non-string object names suppress reads. Empty strings are sent
  // and receive the operation's resource/query error instead.
  return config && typeof config.objectApiName === "string";
}

function compactBody(config) {
  return Object.fromEntries(Object.entries(config).filter(([, value]) => value !== undefined));
}

function listRead(operation, extraValid = () => true) {
  return createFetchWireAdapter(`/lightning/wire/${operation}`, (config = {}) => {
    return objectConfig(config) && extraValid(config) ? compactBody(config) : null;
  });
}

export const getListInfoByName = listRead("getListInfoByName", config =>
  typeof config.listViewApiName === "string");
export const getListRecordsByName = createFetchWireAdapter("/lightning/wire/getListRecordsByName", (config = {}) => {
  if (!objectConfig(config) || typeof config.listViewApiName !== "string") return null;
  const body = compactBody(config);
  // Captured string sortBy values are not a selection for this adapter.
  if (typeof body.sortBy === "string") delete body.sortBy;
  return body;
});
export const getListInfosByObjectName = listRead("getListInfosByObjectName");
export const getListInfosByName = createFetchWireAdapter("/lightning/wire/getListInfosByName", (config = {}) => {
  if (Object.hasOwn(config, "names") && config.names === undefined) return null;
  return { names: Array.isArray(config.names) ? config.names : [] };
});
export const getListPreferences = listRead("getListPreferences", config =>
  typeof config.listViewApiName === "string");

function write(operation, config) {
  if (!config || typeof config !== "object" || Array.isArray(config) ||
      typeof config.objectApiName !== "string" || typeof config.listViewApiName !== "string") {
    return Promise.reject(new Error(`Invalid config for "${operation}"`));
  }
  return fetch(`/lightning/wire/${operation}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(compactBody(config)),
  }).then(response => response.json()).then(payload => {
    if (payload.error) throw payload.error;
    if (operation === "deleteListInfo") return undefined;
    return listMutationRepresentation(operation, payload.data);
  });
}
export function createListInfo(config) { return write("createListInfo", config); }
export function updateListInfoByName(config) { return write("updateListInfoByName", config); }
export function deleteListInfo(config) { return write("deleteListInfo", config); }
export function updateListPreferences(config) { return write("updateListPreferences", config); }

// Public result-selection descriptors captured at both supported API floors.
// Identity and data come from the selected local view, never from a fixture.
const listInfoSelection = {
  "kind": "Fragment",
  "private": [
    "eTag"
  ],
  "selections": [
    {
      "kind": "Scalar",
      "name": "cloneable"
    },
    {
      "kind": "Scalar",
      "name": "createable"
    },
    {
      "kind": "Scalar",
      "name": "deletable"
    },
    {
      "fragment": {
        "kind": "Fragment",
        "opaque": true,
        "private": [],
        "version": "64e2cac6d374ad92f491a9f8a952c6c3"
      },
      "kind": "Link",
      "name": "displayColumns",
      "plural": true
    },
    {
      "kind": "Scalar",
      "name": "filterLogicString"
    },
    {
      "kind": "Object",
      "name": "filteredByInfo",
      "plural": true,
      "selections": [
        {
          "kind": "Scalar",
          "name": "fieldApiName"
        },
        {
          "kind": "Scalar",
          "name": "label"
        },
        {
          "kind": "Scalar",
          "name": "operandLabels",
          "plural": true
        },
        {
          "kind": "Scalar",
          "name": "operator"
        }
      ]
    },
    {
      "kind": "Scalar",
      "name": "hasMassActions",
      "required": false
    },
    {
      "kind": "Object",
      "name": "inlineEditDetails",
      "selections": [
        {
          "kind": "Scalar",
          "name": "message"
        },
        {
          "kind": "Scalar",
          "name": "state"
        }
      ]
    },
    {
      "kind": "Scalar",
      "name": "label"
    },
    {
      "kind": "Object",
      "name": "listReference",
      "selections": [
        {
          "kind": "Scalar",
          "name": "id"
        },
        {
          "kind": "Scalar",
          "name": "listViewApiName"
        },
        {
          "kind": "Scalar",
          "name": "objectApiName"
        },
        {
          "kind": "Scalar",
          "name": "type"
        }
      ]
    },
    {
      "kind": "Object",
      "name": "listShares",
      "plural": true,
      "selections": [
        {
          "kind": "Scalar",
          "name": "shareType"
        },
        {
          "kind": "Object",
          "name": "shares",
          "plural": true,
          "selections": [
            {
              "kind": "Scalar",
              "name": "label"
            },
            {
              "kind": "Scalar",
              "name": "shareApiName"
            }
          ]
        }
      ]
    },
    {
      "kind": "Scalar",
      "name": "objectApiNames",
      "plural": true
    },
    {
      "kind": "Object",
      "name": "orderedByInfo",
      "plural": true,
      "selections": [
        {
          "kind": "Scalar",
          "name": "fieldApiName"
        },
        {
          "kind": "Scalar",
          "name": "isAscending"
        },
        {
          "kind": "Scalar",
          "name": "label"
        }
      ]
    },
    {
      "kind": "Object",
      "name": "scope",
      "nullable": true,
      "selections": [
        {
          "kind": "Scalar",
          "name": "apiName"
        },
        {
          "kind": "Object",
          "name": "entity",
          "nullable": true,
          "selections": [
            {
              "kind": "Scalar",
              "name": "id"
            },
            {
              "kind": "Scalar",
              "name": "label"
            }
          ]
        },
        {
          "kind": "Scalar",
          "name": "label"
        },
        {
          "kind": "Object",
          "name": "relatedEntity",
          "nullable": true,
          "selections": [
            {
              "kind": "Scalar",
              "name": "id"
            },
            {
              "kind": "Scalar",
              "name": "label"
            },
            {
              "kind": "Scalar",
              "name": "type"
            }
          ]
        }
      ]
    },
    {
      "kind": "Scalar",
      "name": "searchable"
    },
    {
      "kind": "Scalar",
      "name": "updateable"
    },
    {
      "kind": "Object",
      "name": "userPreferences",
      "selections": [
        {
          "kind": "Scalar",
          "map": true,
          "name": "columnWidths"
        },
        {
          "kind": "Scalar",
          "map": true,
          "name": "columnWrap"
        }
      ]
    },
    {
      "kind": "Scalar",
      "name": "visibility"
    },
    {
      "kind": "Scalar",
      "name": "visibilityEditable"
    }
  ],
  "version": "5adf17d38be03c0f71b1c52485b377a6"
};
const listPreferencesSelection = {
  "kind": "Fragment",
  "private": [],
  "selections": [
    {
      "kind": "Scalar",
      "map": true,
      "name": "columnWidths"
    },
    {
      "kind": "Scalar",
      "map": true,
      "name": "columnWrap"
    },
    {
      "kind": "Object",
      "name": "listReference",
      "selections": [
        {
          "kind": "Scalar",
          "name": "id"
        },
        {
          "kind": "Scalar",
          "name": "listViewApiName"
        },
        {
          "kind": "Scalar",
          "name": "objectApiName"
        },
        {
          "kind": "Scalar",
          "name": "type"
        }
      ]
    },
    {
      "kind": "Object",
      "name": "orderedBy",
      "plural": true,
      "selections": [
        {
          "kind": "Scalar",
          "name": "fieldApiName"
        },
        {
          "kind": "Scalar",
          "name": "isAscending"
        },
        {
          "kind": "Scalar",
          "name": "label"
        }
      ]
    }
  ],
  "version": "458d4a6a30201e422e8daec5fcb03845"
};
function listMutationRepresentation(operation, data) {
  const reference = data.listReference;
  const preferences = operation === "updateListPreferences";
  const recordId = preferences
    ? `UiApi::ListPreferencesRepresentation:${reference.objectApiName}:${reference.listViewApiName}`
    : `UiApi::ListInfoRepresentation:${reference.listViewApiName.toLowerCase()}:${reference.objectApiName.toLowerCase()}:listView`;
  const collection = () => ({ set: {}, valueMap: {} });
  return {
    data, missingLinks: collection(), missingPaths: collection(), seenRecords: collection(),
    recordId, refresh: undefined, state: "Fulfilled", variables: {},
    select: { node: preferences ? listPreferencesSelection : listInfoSelection, recordId, variables: {} },
  };
}
