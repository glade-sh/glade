import assert from "node:assert/strict";
import test from "node:test";

test("API 67: distinct MessageContexts in one page receive default-scope messages", async (t) => {
  const priorGlobals = new Map(["window", "document", "CustomEvent"].map((key) => [
    key,
    { present: Object.prototype.hasOwnProperty.call(globalThis, key), value: globalThis[key] },
  ]));
  const dispatchedEvents = [];
  // Both components use this one window/document realm: one active page, no other tabs.
  globalThis.window = {};
  globalThis.document = { dispatchEvent: (event) => dispatchedEvents.push(event) };
  globalThis.CustomEvent = class CustomEvent {
    constructor(type, options) {
      this.type = type;
      this.detail = options?.detail;
    }
  };
  let resetMessages = () => {};
  t.after(() => {
    resetMessages();
    for (const [key, prior] of priorGlobals) {
      if (prior.present) globalThis[key] = prior.value;
      else delete globalThis[key];
    }
  });

  const {
    APPLICATION_SCOPE, MessageContext, clearMessages, createMessageContext,
    publish, releaseMessageContext, subscribe, unsubscribe,
  } = await import("../src/shell/message-service.mjs");
  resetMessages = clearMessages;
  clearMessages();

  const publisher = new MessageContext();
  const subscriber = new MessageContext();
  const applicationContext = createMessageContext();
  assert.notEqual(publisher.context, subscriber.context);
  const channel = { name: "ScopeChannel" };
  const sameContext = [];
  const distinctContext = [];
  const applicationScope = [];
  const sameSubscription = subscribe(publisher.context, channel, (value) => sameContext.push(value));
  subscribe(subscriber.context, channel, (value) => distinctContext.push(value));
  subscribe(applicationContext, channel, (value) => applicationScope.push(value), { scope: APPLICATION_SCOPE });

  publish(publisher.context, channel, { step: 1 });
  const afterFirstPublish = {
    sameContext: [...sameContext],
    distinctContext: [...distinctContext],
    applicationScope: [...applicationScope],
  };

  unsubscribe(sameSubscription);
  publish(publisher.context, channel, { step: 2 });
  const afterUnsubscribe = {
    sameContext: [...sameContext],
    distinctContext: [...distinctContext],
    applicationScope: [...applicationScope],
  };

  subscriber.disconnect();
  releaseMessageContext(applicationContext);
  publish(publisher.context, channel, { step: 3 });

  assert.deepEqual(afterFirstPublish.sameContext, [{ step: 1 }]);
  assert.deepEqual(afterFirstPublish.applicationScope, [{ step: 1 }]);
  assert.deepEqual(afterUnsubscribe.sameContext, [{ step: 1 }]);
  assert.deepEqual(afterUnsubscribe.applicationScope, [{ step: 1 }, { step: 2 }]);
  assert.deepEqual(sameContext, [{ step: 1 }], "unsubscribe must stop same-context delivery");
  assert.deepEqual(applicationScope, [{ step: 1 }, { step: 2 }], "release must stop application-scope delivery");
  assert.equal(dispatchedEvents.length, 3);

  assert.deepEqual(afterFirstPublish.distinctContext, [{ step: 1 }],
    "a second MessageContext in the same page must receive the default-scope message");
  assert.deepEqual(afterUnsubscribe.distinctContext, [{ step: 1 }, { step: 2 }]);
  assert.deepEqual(distinctContext, [{ step: 1 }, { step: 2 }],
    "disconnect must release the second context's subscription");
});
