import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import sanitizeHTML from "./purifyLib.mjs";

// Native control_rich_tag_* distinguishes discarded content from unwrapped
// object/custom elements.
const blockedTags = new Set(["script", "style", "iframe", "embed", "link", "meta", "base", "template", "svg", "math"]);
const presentationAttributes = new Set(["class", "title", "alt", "target", "width", "height", "colspan", "rowspan", "id", "style", "aria-label"]);

function safeAttributes(node) {
  const attrs = {};
  for (const { name, value } of node.attributes) {
    // control_rich_other_attributes exercises the HTML data-* attribute path.
    if (presentationAttributes.has(name) || name.startsWith("data-")) {
      attrs[name] = value;
    } else if ((name === "href" || name === "src") && /^https?:/i.test(value)) {
      // Native malformed HTTP href/src controls retain the text verbatim.
      attrs[name] = value;
    } else if (name === "src" && node.localName === "img" && /^data:image\/gif;base64,/i.test(value)) {
      // control_rich_src_data establishes this image media type only.
      attrs[name] = value;
    } else if (name === "href" || name === "src") {
      try {
        const protocol = new URL(value, document.baseURI).protocol;
        if (["http:", "https:", "mailto:", "tel:"].includes(protocol)) attrs[name] = value;
      } catch {
        // Invalid URLs do not enter the rendered HTML.
      }
    }
  }
  return attrs;
}

function renderRichContent($api, value) {
  const fragment = document.createElement("template");
  fragment.innerHTML = sanitizeHTML(value);
  let key = 1;
  function renderNodes(nodes) {
    return $api.i(Array.from(nodes), node => {
      if (node.nodeType === 3) return $api.t(node.textContent);
      if (node.nodeType !== 1 || blockedTags.has(node.localName)) return null;
      const children = renderNodes(node.childNodes);
      if (node.localName === "object" || node.localName.includes("-")) return children;
      return $api.h(node.localName, { key: key++, attrs: safeAttributes(node) }, children);
    });
  }
  return renderNodes(fragment.content.childNodes);
}

function renderFormattedRichText($api, $cmp) {
  return [$api.h("span", { key: 0 }, renderRichContent($api, $cmp.value))];
}
renderFormattedRichText.stylesheets = [];
const template = registerTemplate(renderFormattedRichText);
freezeTemplate(template);

class FormattedRichText extends LightningElement {
  constructor() {
    super();
    this._value = "";
  }

  get value() {
    return this._value;
  }

  set value(value) {
    this._value = value == null ? "" : String(value);
  }

  connectedCallback() {
    this.classList.add("slds-rich-text-editor__output");
  }
}
registerDecorators(FormattedRichText, {
  publicProps: { value: { config: 3 } },
  fields: ["_value"],
});
export default registerComponent(FormattedRichText, {
  tmpl: template,
  sel: "lightning-formatted-rich-text",
  apiVersion: 63,
});
