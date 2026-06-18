import { registerDecorators as _registerDecorators, LightningElement, registerComponent as _registerComponent } from "lwc";
import _tmpl from "./verticalNavigationItem.html.js";
const DEFAULT_HREF = 'javascript:void(0);'; // eslint-disable-line no-script-url

/**
 * A text-only link within lightning-vertical-navigation-section or lightning-vertical-navigation-overflow.
 */
class LightningVerticalNavigationItem extends LightningElement {
  constructor(...args) {
    super(...args);
    /**
     * The text displayed for the navigation item.
     * @type {string}
     * @required
     */
    this.label = void 0;
    /**
     * A unique identifier for the navigation item.
     * The name is used by the `select` event on lightning-vertical-navigation to identify which item is selected.
     * @type {string}
     * @required
     */
    this.name = void 0;
    /**
     * The URL of the page that the navigation item goes to.
     * @type {string}
     */
    this.href = DEFAULT_HREF;
    this.state = {
      selected: false
    };
  }
  connectedCallback() {
    this.setAttribute('role', 'listitem');
    this.classList.add('slds-nav-vertical__item');
    this.dispatchEvent(new CustomEvent('privateitemregister', {
      bubbles: true,
      cancelable: true,
      composed: true,
      detail: {
        callbacks: {
          select: this.select.bind(this),
          deselect: this.deselect.bind(this)
        },
        name: this.name
      }
    }));
  }
  select() {
    this.state.selected = true;
    this.classList.add('slds-is-active');
  }
  deselect() {
    this.state.selected = false;
    this.classList.remove('slds-is-active');
  }
  get ariaCurrent() {
    return this.state.selected ? 'page' : null;
  }
  handleClick(event) {
    this.dispatchEvent(new CustomEvent('privateitemselect', {
      bubbles: true,
      cancelable: true,
      composed: true,
      detail: {
        name: this.name
      }
    }));
    if (this.href === DEFAULT_HREF) {
      event.preventDefault();
    }
  }
  /*LWC compiler v8.20.4*/
}
_registerDecorators(LightningVerticalNavigationItem, {
  publicProps: {
    label: {
      config: 0
    },
    name: {
      config: 0
    },
    href: {
      config: 0
    }
  },
  track: {
    state: 1
  }
});
const __lwc_component_class_internal = _registerComponent(LightningVerticalNavigationItem, {
  tmpl: _tmpl,
  sel: "lightning-vertical-navigation-item",
  apiVersion: 63
});
export default __lwc_component_class_internal;