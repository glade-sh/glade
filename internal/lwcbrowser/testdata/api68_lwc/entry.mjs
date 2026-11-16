import {createElement} from 'lwc';
import CheckboxGroup from '/shim/checkboxGroup.js';
import InputAddress from '/shim/inputAddress.js';
import Slider from '/shim/slider.js';
function add(tag, Ctor, props = {}) {
  const el = createElement(tag, {is: Ctor});
  Object.assign(el, props);
  document.body.appendChild(el);
  return el;
}
const item = location.search.slice(1);
// The probe markup of captures-lwc cap68<Item>, as component properties.
if (item === 'b01') {
  add('lightning-checkbox-group', CheckboxGroup, {name: 'choices', label: 'Choices', required: true,
    options: [{label: 'First long choice', value: 'first'}, {label: 'Second long choice', value: 'second'}], value: []});
}
if (item === 'b17') {
  add('lightning-input-address', InputAddress, {addressLabel: 'Delivery destination', streetLabel: 'Street', cityLabel: 'City',
    provinceLabel: 'State', postalCodeLabel: 'Postal code', countryLabel: 'Country', showAddressLookup: true});
}
if (item === 'b24') {
  for (const step of [0.1, 0.25, 0.05]) add('lightning-slider', Slider, {label: 'Step ' + step, min: 0, max: 100, step, value: 50});
}
requestAnimationFrame(() => { window.__ready = true; });
