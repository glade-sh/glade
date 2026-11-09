import assert from 'node:assert/strict';
let input='';for await(const chunk of process.stdin) input+=chunk;
const sources=JSON.parse(input), url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const lwc=url('export class LightningElement extends EventTarget {} export function registerDecorators(C,x){C.metadata=x;return C} export const registerTemplate=x=>x;export const freezeTemplate=()=>{};export function registerComponent(C,x){return C}');
const diag=url('export function reportDiagnostic() {}');
for(const [name,source] of Object.entries(sources)) {
 const C=(await import(url(source.replaceAll('"lwc"',JSON.stringify(lwc)).replaceAll('"@glade/shell/diagnostics"',JSON.stringify(diag))))).default;
 for(const prop of ['headingLevel','illustrationName','iconAlternativeText']) {
  const expected=(name==='emptyState' && prop!=='iconAlternativeText') || (name==='richTextToolbarButton' && prop==='iconAlternativeText');
  assert.equal(Boolean(C.metadata.publicProps[prop]),expected,`${name}.${prop}`);
 }
}
