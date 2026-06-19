function sourceFromTemplate(strings, values) {
  if (Array.isArray(strings) && Object.prototype.hasOwnProperty.call(strings, "raw")) {
    return strings.reduce((out, chunk, index) => {
      const value = index < values.length ? values[index] : "";
      return out + chunk + String(value ?? "");
    }, "");
  }
  return String(strings || "");
}

export function gql(strings, ...values) {
  const source = sourceFromTemplate(strings, values);
  return {
    kind: "Document",
    definitions: [],
    source,
    loc: { source: { body: source, name: "GraphQL request" } },
    toString() {
      return source;
    },
  };
}

function readGraphQLData() {
  if (typeof document === "undefined") {
    return {};
  }
  const node = document.getElementById("glade-lwc-graphql");
  if (!node) {
    return {};
  }
  try {
    const payload = JSON.parse(node.textContent || "{}");
    return payload && typeof payload.data === "object" ? payload.data : {};
  } catch (_err) {
    return {};
  }
}

export function graphql() {
  return Promise.resolve({
    data: readGraphQLData(),
    errors: [],
  });
}

export function query(documentOrConfig, variables = {}) {
  const config = documentOrConfig && documentOrConfig.query ? documentOrConfig : {
    query: documentOrConfig,
    variables,
  };
  return graphql(config);
}

export default graphql;
