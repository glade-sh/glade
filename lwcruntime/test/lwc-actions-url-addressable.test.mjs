import assert from "node:assert/strict";
import test from "node:test";
import { requireLWCToolchain, startLWCDevServer } from "./helpers.mjs";

const fixture = "testdata/local-tests/lwc-shell";

test("LWC shell serves URL-addressable and quick action contexts", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }

  const server = await startLWCDevServer(t, {
    projectRel: fixture,
    pagePath: "/lwc/preview/cmp/c/actionProbe?c__name=value",
  });
  if (!server) {
    return;
  }

  try {
    const urlAddressable = await fetch(`${server.baseURL}/lwc/preview/cmp/c/actionProbe?c__name=value`);
    assert.equal(urlAddressable.status, 200);
    const urlHTML = await urlAddressable.text();
    assert.match(urlHTML, /"componentName":"c:actionProbe"/);
    assert.match(urlHTML, /"c__name":"value"/);
    assert.match(urlHTML, /"standard__component"/);

    const recordAction = await fetch(`${server.baseURL}/lwc/preview/action/Account/001000000000001AAA/Update_Status`);
    assert.equal(recordAction.status, 200);
    const recordHTML = await recordAction.text();
    assert.match(recordHTML, /"recordId":"001000000000001AAA"/);
    assert.match(recordHTML, /"objectApiName":"Account"/);
    assert.match(recordHTML, /"actionName":"Account.Update_Status"/);
    assert.match(recordHTML, /"actionType":"ScreenAction"/);
    assert.match(recordHTML, /"standard__quickAction"/);

    const globalAction = await fetch(`${server.baseURL}/lwc/preview/action/global/Global_Status`);
    assert.equal(globalAction.status, 200);
    const globalHTML = await globalAction.text();
    assert.match(globalHTML, /"actionName":"Global_Status"/);
    assert.match(globalHTML, /"actionType":"ScreenAction"/);
  } finally {
    await server.close();
  }
});
