const item = location.search.slice(1).split('&')[0];
// B25 measures real tab widths, so it runs under synthetic shadow with the
// SLDS stylesheet, as the local shell does.
if (item === 'b25') {
  await import('@lwc/synthetic-shadow');
  const {loadSLDS} = await import('@glade/slds');
  const style = await loadSLDS();
  if (!style.ok) throw new Error('SLDS stylesheet unavailable');
}
const {createElement} = await import('lwc');
async function load(name) {
  return (await import('/shim/' + name + '.js')).default;
}
async function add(tag, name, props = {}, parent = document.body) {
  const el = createElement(tag, {is: await load(name)});
  Object.assign(el, props);
  parent.appendChild(el);
  return el;
}
const frame = () => new Promise(resolve => requestAnimationFrame(() => resolve()));
// The probe markup of captures-lwc cap68<Item>, as component properties.
if (item === 'b01') {
  await add('lightning-checkbox-group', 'checkboxGroup', {name: 'choices', label: 'Choices', required: true,
    options: [{label: 'First long choice', value: 'first'}, {label: 'Second long choice', value: 'second'}], value: []});
}
if (item === 'b17') {
  await add('lightning-input-address', 'inputAddress', {addressLabel: 'Delivery destination', streetLabel: 'Street', cityLabel: 'City',
    provinceLabel: 'State', postalCodeLabel: 'Postal code', countryLabel: 'Country', showAddressLookup: true});
}
if (item === 'b24') {
  for (const step of [0.1, 0.25, 0.05]) await add('lightning-slider', 'slider', {label: 'Step ' + step, min: 0, max: 100, step, value: 50});
}
if (item === 'b07') {
  const rows = [{id: 'r1', name: 'Example row'}, {id: 'r2', name: 'Second row'}];
  const columns = [{label: 'Name', fieldName: 'name', type: 'text'}];
  const error = extra => ({rows: {r1: {title: 'Capture validation', messages: ['Controlled fixture message'], fieldNames: ['name'], ...extra}}});
  const table = {keyField: 'id', columns, showRowNumberColumn: true};
  await add('lightning-datatable', 'datatable', {...table, data: rows, errors: error({rowTitle: 'Capture row title'})});
  await add('lightning-datatable', 'datatable', {...table, data: rows, errors: error({})});
  await add('lightning-datatable', 'datatable', {...table, data: [{id: 'r1', name: {label: 'Object row'}}], errors: error({})});
}
if (item === 'b11') {
  const input = await add('lightning-input', 'input', {type: 'datetime', label: 'Appointment', required: true, value: 'not-a-datetime'});
  input.reportValidity();
}
if (item === 'b15_en_US' || item === 'b15_fr_FR') {
  for (const [type, label, value] of [['date', 'Date', 'not-a-date'], ['time', 'Time', '99:99'], ['datetime', 'DateTime', 'not-a-datetime']]) {
    const input = await add('lightning-input', 'input', {type, label, value});
    input.reportValidity();
  }
}
if (item === 'b18') {
  // Contact.Email as record-edit-form object info supplies it to the field.
  const field = await add('lightning-input-field', 'inputField', {fieldName: 'Email'});
  field.wireRecordUi({record: {apiName: 'Contact', fields: {}}, objectInfo: {fields: {Email: {apiName: 'Email', dataType: 'Email', label: 'Email', length: 80}}}});
}
if (item === 'b25') {
  const holder = document.createElement('div');
  holder.style.width = '260px';
  document.body.appendChild(holder);
  const tabset = await add('lightning-tabset', 'tabset', {}, holder);
  const names = ['First', 'Second', 'Third', 'Fourth', 'Fifth', 'Sixth', 'Seventh', 'Eighth', 'Ninth', 'Tenth', 'Eleventh', 'Twelfth'];
  for (const name of names) {
    const tab = await add('lightning-tab', 'tab', {label: name + ' long tab'}, tabset);
    tab.appendChild(document.createTextNode(name));
  }
  for (let i = 0; i < 4; i++) await frame();
}
await frame();
window.__ready = true;
