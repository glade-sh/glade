export function classSet(initial = "") {
  const values = new Set(String(initial || "").split(/\s+/).filter(Boolean));
  return {
    add(value) {
      if (typeof value === "string") {
        values.add(value);
      }
      if (value && typeof value === "object") {
        for (const [key, enabled] of Object.entries(value)) {
          if (enabled) {
            values.add(key);
          }
        }
      }
      return this;
    },
    invert() {
      return this;
    },
    toString() {
      return Array.from(values).join(" ");
    },
  };
}

export function queryFocusable(root) {
  return Array.from(root?.querySelectorAll?.("a,button,input,select,textarea,[tabindex]") || []);
}

export function formatLabel(label, ...args) {
  return String(label || "").replace(/\{(\d+)\}/g, (_match, index) => String(args[Number(index)] ?? ""));
}

export function linkTextNodes(value) {
  return value;
}

export function formatUrl(value) {
  return String(value || "");
}
