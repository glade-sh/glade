import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { repoRoot, startLWCDevServer } from "./helpers.mjs";

for (const apiVersion of ["62.0", "67.0"]) {
  test(`LWC shell object search retains source security grants at API ${apiVersion}`, async (t) => {
    const projectRoot = fs.mkdtempSync(path.join(os.tmpdir(), "glade-object-search-"));
    const write = (name, contents) => {
      const file = path.join(projectRoot, name);
      fs.mkdirSync(path.dirname(file), { recursive: true });
      fs.writeFileSync(file, contents);
    };
    const metadata = "force-app/main/default";
    write("sfdx-project.json", JSON.stringify({
      packageDirectories: [{ path: "force-app", default: true }],
      sourceApiVersion: apiVersion,
    }));
    for (const name of ["SearchProbeOne", "SearchProbeTwo"]) {
      write(`${metadata}/classes/${name}.cls`, `public class ${name} {}`);
      write(`${metadata}/classes/${name}.cls-meta.xml`,
        `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>${apiVersion}</apiVersion><status>Active</status></ApexClass>`);
      write(`${metadata}/pages/${name}.page`, "<apex:page>Search preview</apex:page>");
      write(`${metadata}/pages/${name}.page-meta.xml`,
        `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>${apiVersion}</apiVersion><label>${name}</label></ApexPage>`);
    }
    write(`${metadata}/lwc/searchProbe/searchProbe.js`,
      'import { LightningElement } from "lwc"; export default class SearchProbe extends LightningElement {}');
    write(`${metadata}/lwc/searchProbe/searchProbe.html`, "<template><p>Search preview</p></template>");
    write(`${metadata}/lwc/searchProbe/searchProbe.js-meta.xml`,
      `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>${apiVersion}</apiVersion><isExposed>true</isExposed><targets><target>lightning__RecordPage</target></targets></LightningComponentBundle>`);
    write("data/accounts.json", JSON.stringify([{
      attributes: { type: "Account" },
      Id: "001000000000001AAA",
      Name: "Search Preview Account",
    }]));

    let server;
    try {
      server = await startLWCDevServer(t, {
        projectRel: path.relative(repoRoot, projectRoot),
        pagePath: "/lwc/builder",
      });
      assert.ok(server, "synthetic project server started");
      const response = await fetch(`${server.baseURL}/lightning/local/objects.json?q=acc`);
      assert.equal(response.status, 200);
      const { objects } = await response.json();
      const account = objects.find((object) => object.apiName === "Account");
      const access = objects.find((object) => object.apiName === "SetupEntityAccess");
      assert.ok(account, "Account remains searchable");
      assert.ok(access, "source security grants remain searchable");
      assert.equal(account.recordCount, 1);
      assert.equal(access.recordCount, 4, "administrator grants cover both classes and both pages");
      assert.equal(objects[0].apiName, access.apiName, "populated objects sort by record count");
      assert.ok(objects.indexOf(access) < objects.indexOf(account));
    } finally {
      await server?.close();
      fs.rmSync(projectRoot, { recursive: true, force: true });
    }
  });
}
