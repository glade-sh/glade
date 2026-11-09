import assert from 'node:assert/strict';let input='';for await(const c of process.stdin)input+=c;const data=JSON.parse(input);
const url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const service=url('export const CurrentPageReferenceAdapter = class {}; export function readShellContext(){return {}} export async function generatePageReferenceUrl() {} export async function navigate() {}');
const mod=await import(url(data.source.replace('"/lightning/runtime/shell/navigation-service.js"',JSON.stringify(service))));
assert.ok(Object.hasOwn(mod,'navigate'),'named navigate binding is missing');assert.ok(Object.hasOwn(mod,'NavigationContext'),'named NavigationContext binding is missing');
assert.deepEqual({navigate:typeof mod.navigate,context:typeof mod.NavigationContext},JSON.parse(data.expected));
