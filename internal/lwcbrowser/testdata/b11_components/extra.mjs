import assert from 'node:assert/strict';let input='';for await(const c of process.stdin)input+=c;const data=JSON.parse(input), native=JSON.parse(data.expected);
const url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const lwc=url(`export class LightningElement extends EventTarget {} export function registerDecorators(C,x){C.metadata=x;return C} export const registerTemplate=x=>x;export const freezeTemplate=()=>{};export function registerComponent(C,x){C.template=x.tmpl;return C}`);
const diag=url('export function reportDiagnostic() {}');const source=data.source.replaceAll('"lwc"',JSON.stringify(lwc)).replaceAll('"@glade/shell/diagnostics"',JSON.stringify(diag));const C=(await import(url(source))).default;
const api={h:(tag,data,children)=>({tag,data,children}),t:text=>({text}),d:text=>String(text??''),b:fn=>fn,s:(name,data,children,slots)=>({tag:'slot',name,data,children:slots[name]||children})};
function render(c,slots={}) {return C.template(api,c,slots,{})}
function find(nodes,pred){for(const n of nodes.flat(Infinity)){if(!n)continue;if(pred(n))return n;const found=find(n.children||[],pred);if(found)return found;}return null;}
if(data.case==='empty'){
 for(const prop of ['headingLevel','illustrationName','alternativeText','size'])assert.ok(C.metadata.publicProps[prop],`missing public property ${prop}`);
 for(const [i,props] of native.initial.entries()){let c=new C();for(const [k,v] of Object.entries(props))if(v!=='undefined')c[k]=v;const nodes=render(c);let heading=find(nodes,n=>n.tag==='h3');assert.equal(heading.data.attrs?.['aria-level']??null,i===0?'2':null);if(i===0)assert.ok(find(nodes,n=>n.tag==='svg'||n.tag==='img'||n.tag==='lightning-illustration'),'missing illustration');}
}else{
 for(const name of ['showPopup','closePopup'])assert.ok(C.metadata.publicMethods.includes(name),`missing public method ${name}`);
 let c=new C();c.connectedCallback();assert.equal(c.selected,native.initial.selected);const slots={'':[{text:'Popup content'}]};assert.equal(c.showPopup(),undefined);assert.ok(find(render(c,slots),n=>n.text==='Popup content'));assert.equal(c.closePopup(),undefined);assert.equal(find(render(c,slots),n=>n.tag==='div' && n.data.attrs.role==='dialog').data.attrs.hidden,'');
}
