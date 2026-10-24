import assert from 'node:assert/strict';
import fs from 'node:fs';
let input = '';
for await (const chunk of process.stdin) input += chunk;
const {source, expected, api} = JSON.parse(input);
const url = 'data:text/javascript,' + encodeURIComponent(source);
const nativeSource = fs.readFileSync(new URL(`./project/force-app/main/default/lwc/deviceProbe${api}/deviceProbe${api}.js`,import.meta.url),'utf8');
const componentSource = nativeSource
    .replace("import { LightningElement } from 'lwc';", 'class LightningElement {}')
    .replace("'lightning/mobileCapabilities'", JSON.stringify(url));
try {
    const {default: Probe} = await import('data:text/javascript,' + encodeURIComponent(componentSource));
    const probe = new Probe();
    await probe.connectedCallback();
    assert.deepEqual(JSON.parse(probe.result), JSON.parse(expected));
} catch (error) {
    console.error(`${error.name}: ${error.message.replace(/'data:text[^']*'/g, "'<module>'")}`);
    process.exitCode = 1;
}
