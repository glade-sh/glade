package embed

// ManualDOMJS preserves the native nullish textContent assignments captured in
// d_manual_null and d_manual_undefined at API 59 and 67. Only synthetic
// manual DOM elements coerce these values; other nodes and values keep the
// synthetic shadow setter's behavior.
const ManualDOMJS = `import "@lwc/synthetic-shadow";
{
  const text = Object.getOwnPropertyDescriptor(Node.prototype, "textContent");
  Object.defineProperty(Node.prototype, "textContent", {
    ...text,
    set(value) {
      if ((value === null || value === undefined) &&
          this instanceof Element && this.$domManual$ === true) {
        value = String(value);
      }
      text.set.call(this, value);
    },
  });
}
`
