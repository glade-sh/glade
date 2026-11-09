import assert from 'node:assert/strict';
import test from 'node:test';
import { gql, graphql, executeMutation } from '../src/lightning/graphql.mjs';
globalThis.window = {};
const { buildLocalUrl, navigateToUrl } = await import('../src/shell/navigation-service.mjs');

test('B11 proxied documents retain their query and mutation boundaries', async () => {
  globalThis.window = {};
  const query = gql`query { __typename }`;
  assert.deepEqual(await graphql({ query: new Proxy(query, {}) }), await graphql({ query }));
  delete globalThis.window;
});

test('B11 mutation documents never execute in the query adapter', async () => {
  globalThis.window = {};
  const mutation = gql`mutation { __typename }`;
  for (const doc of [mutation, new Proxy(mutation, {})]) {
    const result = await graphql({ query: doc });
    assert.equal(result.errors?.[0]?.errorType, 'localBoundary');
    assert.equal((await executeMutation({ query: doc })).errors?.[0]?.errorType, 'localBoundary');
    const seen = [];
    const adapter = new graphql(value => seen.push(value));
    adapter.connect(); adapter.update({ query: doc });
    if (adapter.pending) await adapter.pending;
    assert.equal(seen.at(-1)?.errors?.[0]?.errorType, 'localBoundary');
  }
  delete globalThis.window;
});

test('B11 LWR community navigation preserves host mode and replace', () => {
  globalThis.document = { getElementById: id => id === 'glade-lwc-context' ? {textContent: JSON.stringify({community:{site:'Example',lwr:true}})} : null };
  const calls = [];
  globalThis.window = {location:{ pathname:'/start', search:'', assign: url => calls.push(['assign',url]), replace: url => calls.push(['replace',url]) }};
  for (const ref of [
    {type:'standard__namedPage',attributes:{pageName:'home'}},
    {type:'standard__component',attributes:{componentName:'c__Example'}},
    {type:'standard__recordPage',attributes:{recordId:'001',objectApiName:'Account'}},
    {type:'comm__namedPage',attributes:{name:'Home'}},
    {type:'comm__loginPage',attributes:{actionName:'login'}},
    {type:'comm__managedContentPage',attributes:{contentKey:'welcome'}},
    {type:'comm__recordPage',attributes:{recordId:'001',objectApiName:'Account'}},
    {type:'comm__recordRelationshipPage',attributes:{recordId:'001',objectApiName:'Account',relationshipApiName:'Contacts'}}
  ]) assert.equal(new URL(buildLocalUrl(ref),'https://example.test').searchParams.get('lwr'),'true',ref.type);
  delete globalThis.document; delete globalThis.window;
});

test('B11 navigation honors replace', () => {
  const calls = [];
  globalThis.window = {location:{ pathname:'/start', search:'', assign: url => calls.push(['assign',url]), replace: url => calls.push(['replace',url]) }};
  navigateToUrl('/next', {replace:true});
  assert.deepEqual(calls, [['replace','/next']]);
  delete globalThis.document; delete globalThis.window;
});

test('B11 opaque document does not expose mutable parser state', async () => {
  globalThis.window = {};
  const doc = gql`query Named { __typename }`;
  for (const symbol of Object.getOwnPropertySymbols(doc)) {
    const exposed = doc[symbol];
    if (exposed && typeof exposed === 'object' && exposed.parsed) exposed.parsed.definitions.length = 0;
  }
  assert.equal((await graphql({query:doc})).data?.__typename, 'Query');
  delete globalThis.window;
});

test('B11 LWR alias and community context survive local navigation', () => {
  const community={site:'Example',isLwr:true,basePath:'/partner',siteId:'site',networkId:'network',guest:true,language:'en-US'};
  globalThis.document={getElementById:id=>id==='glade-lwc-context'?{textContent:JSON.stringify({community})}:null};
  for(const ref of [{type:'standard__flow',attributes:{flowApiName:'Welcome'}},{type:'standard__namedPage',attributes:{pageName:'home'}},{type:'comm__namedPage',attributes:{name:'Home'}}]) {
    const p=new URL(buildLocalUrl(ref),'https://example.test').searchParams;
    for(const [key,value] of Object.entries({lwr:'true',site:'Example',basePath:'/partner',siteId:'site',networkId:'network',guest:'true',language:'en-US'})) assert.equal(p.get(key),value,`${ref.type} ${key}`);
  }
  delete globalThis.document;
});

test('B11 legacy documents cannot mutate modern cached queries', async () => {
  globalThis.window={};
  const {gql:legacyGql}=await import('../src/lightning/uiGraphQLApi.mjs');
  const legacy=legacyGql`query Legacy { __typename }`;
  legacy.definitions.length=0;
  const modern=gql`query Legacy { __typename }`;
  assert.equal((await graphql({query:modern})).data?.__typename,'Query');
  delete globalThis.window;
});

test('B11 cross-site routes do not inherit origin identity', () => {
  const community={site:'Origin',lwr:true,basePath:'/origin',siteId:'origin-site',networkId:'origin-network',guest:true,language:'en-US'};
  globalThis.document={getElementById:id=>id==='glade-lwc-context'?{textContent:JSON.stringify({community})}:null};
  for (const ref of [
    {type:'comm__namedPage',attributes:{site:'Destination',name:'Home'}},
    {type:'comm__recordPage',attributes:{site:'Destination',recordId:'001'}},
    {type:'comm__managedContentPage',attributes:{siteName:'Destination',contentKey:'item'}},
    {type:'standard__flow',attributes:{flowApiName:'Welcome'},state:{site:'Destination',siteId:'other-site'}}
  ]) {
    const params=new URL(buildLocalUrl(ref),'https://example.test').searchParams;
    assert.notEqual(params.get('siteId'),'origin-site');
    assert.equal(params.has('networkId'),false);
    assert.equal(params.has('basePath'),false);
  }
  delete globalThis.document;
});

test('B11 legacy gql keeps stable identity and original stringification', async () => {
  const {gql:legacy}=await import('../src/lightning/uiGraphQLApi.mjs');
  const a=legacy`query Stable { __typename }`;
  const b=legacy`query Stable { __typename }`;
  assert.equal(a,b);
  const original=String(a);
  a.source='changed';
  assert.equal(String(a),original);
});

test('B11 authenticated guest flag is carried explicitly', () => {
  globalThis.document={getElementById:id=>id==='glade-lwc-context'?{textContent:JSON.stringify({community:{lwr:true,guest:false}})}:null};
  assert.equal(new URL(buildLocalUrl({type:'standard__flow',attributes:{flowApiName:'Welcome'}}),'https://example.test').searchParams.get('guest'),'false');
  delete globalThis.document;
});

test('B11 nested app destinations avoid origin context and same-site casing is accepted', () => {
  globalThis.document={getElementById:id=>id==='glade-lwc-context'?{textContent:JSON.stringify({community:{site:'Origin',lwr:true,siteId:'origin-site',networkId:'origin-network',guest:true}})}:null};
  const references=[
    {type:'comm__namedPage',attributes:{site:'Destination',name:'Home'}},
    {type:'comm__loginPage',attributes:{site:'Destination',actionName:'login'}},
    {type:'comm__managedContentPage',attributes:{site:'Destination',contentKey:'content'}},
    {type:'comm__recordPage',attributes:{site:'Destination',recordId:'001'}},
    {type:'comm__recordRelationshipPage',attributes:{site:'Destination',recordId:'001',relationshipApiName:'Contacts'}}
  ];
  for(const pageRef of references) {
    const url=buildLocalUrl({type:'standard__app',attributes:{appTarget:'Example',pageRef}});
    const params=new URL(url,'https://example.test').searchParams;
    assert.equal(params.has('siteId'),false,pageRef.type);
    assert.equal(params.has('networkId'),false,pageRef.type);
  }
  const same=new URL(buildLocalUrl({type:'comm__namedPage',attributes:{site:'origin',name:'Home'}}),'https://example.test').searchParams;
  assert.equal(same.get('lwr'),'true'); assert.equal(same.get('siteId'),'origin-site');
  delete globalThis.document;
});
