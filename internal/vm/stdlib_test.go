package vm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/resource"
	"github.com/open-aer/oaer/internal/storage"
)

func TestExecStringStdlibMethods(t *testing.T) {
	program, err := CompileAnonymous(`
String s = 'AbcDef';
System.assertEquals(6, s.length());
System.assert(s.contains('cD'));
System.assert(s.startsWith('Ab'));
System.assert(s.endsWith('Def'));
System.assertEquals('abcdef', s.toLowerCase());
System.assertEquals('ABCDEF', s.toUpperCase());
String lowerName = 'hello maximillian';
String upperName = 'Hello max';
System.assertEquals('Hello maximillian', lowerName.capitalize());
System.assertEquals('hello max', upperName.uncapitalize());
System.assertEquals('cDef', s.substring(2));
System.assertEquals('cD', s.substring(2, 4));
System.assertEquals('bcD', s.Substring(1, 4));
System.assertEquals('abcdef', s.ToLowerCase());
System.assert(s.containsIgnoreCase('CD'));
System.assert(s.startsWithIgnoreCase('ab'));
System.assert(s.endsWithIgnoreCase('def'));
System.assert(s.equals('AbcDef'));
System.assert('0'.equals(0));
System.assert(!'0'.equals(null));
System.assertEquals(-1, s.compareTo('B'));
System.assertEquals('Abc', s.left(3));
System.assertEquals('Def', s.right(3));
System.assertEquals('  AbcDef', s.leftPad(8));
System.assertEquals('xyAbcDef', s.leftPad(8, 'xy'));
System.assertEquals('AbcDef  ', s.rightPad(8));
System.assertEquals('AbcDefxy', s.rightPad(8, 'xy'));
System.assertEquals(' AbcDef ', s.center(8));
System.assertEquals('cDe', s.mid(2, 3));
System.assertEquals('feDcbA', s.reverse());
String dotted = 'Salesforce.Lightning.platform';
System.assertEquals('Lightning.platform', dotted.substringAfter('.'));
System.assertEquals('platform', dotted.substringAfterLast('.'));
System.assertEquals('Salesforce', dotted.substringBefore('.'));
System.assertEquals('Salesforce.Lightning', dotted.substringBeforeLast('.'));
String force = 'Salesforce and force.com';
System.assertEquals('Sales and .com', force.remove('force'));
System.assertEquals('and force.com', force.removeStart('Salesforce '));
System.assertEquals('and force.com', force.removeStartIgnoreCase('SALESFORCE '));
System.assertEquals('Salesforce and force', force.removeEnd('.com'));
System.assertEquals('Salesforce and force', force.removeEndIgnoreCase('.COM'));
String accent = 'ÄbcDEF';
System.assertEquals('bcDEF', accent.removeStartIgnoreCase('ä'));
System.assertEquals('Äbc', accent.removeEndIgnoreCase('def'));
String spaced = ' a b c ';
System.assertEquals('abc', spaced.deleteWhitespace());
String manySpaces = ' a   b  c ';
System.assertEquals('a b c', manySpaces.normalizeSpace());
String ab = 'ab';
System.assertEquals('ababab', ab.repeat(3));
System.assertEquals('ab|ab|ab', ab.repeat('|', 3));
System.assert(String.isEmpty(null));
System.assert(String.isEmpty(''));
System.assert(!String.isEmpty(' '));
System.assert(String.isNotEmpty('x'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDomDocumentLoadAndXmlNodeMembers(t *testing.T) {
	program, err := CompileAnonymous(`
Dom.Document doc = new Dom.Document();
doc.load('<myprefix:node xmlns:myprefix="http://my.name.space" myprefix:type="test">hello<!--note--><child id="c">there</child></myprefix:node>');
Dom.Document htmlDoc = new Dom.Document();
htmlDoc.load('<div><img src="notmyproblem.jpeg">Hello</div>');
try {
  htmlDoc.load('<div><img src="<<">Hello</div>');
  System.assert(false);
} catch (XmlException e) {
  System.assert(e.getMessage().contains('Dom.Document.load invalid XML'));
}
Dom.XmlNode root = doc.getRootElement();
System.assertEquals(Dom.XmlNodeType.ELEMENT, root.getNodeType());
System.assertEquals('node', root.getName());
System.assertEquals('http://my.name.space', root.getNamespace());
System.assertEquals('myprefix', root.getPrefix());
System.assertEquals('myprefix', root.getPrefixFor('http://my.name.space'));
System.assertEquals('http://my.name.space', root.getNamespaceFor('myprefix'));
System.assertEquals(1, root.getAttributeCount());
System.assertEquals('type', root.getAttributeKeyAt(0));
System.assertEquals('http://my.name.space', root.getAttributeKeyNsAt(0));
System.assertEquals('test', root.getAttribute('type', 'http://my.name.space'));
System.assertEquals('test', root.getAttributeValue('type', 'http://my.name.space'));
System.assertEquals(null, root.getAttributeValueNs('type', 'http://my.name.space'));
System.assertEquals('hellonote', root.getText());
List<Dom.XmlNode> children = root.getChildren();
System.assertEquals(3, children.size());
System.assertEquals(1, root.getChildElements().size());
System.assertEquals(Dom.XmlNodeType.TEXT, children[0].getNodeType());
System.assertEquals(Dom.XmlNodeType.COMMENT, children[1].getNodeType());
System.assertEquals('child', children[2].getName());
System.assertEquals('there', root.getChildElement('child', null).getText());
System.assertEquals(root, children[2].getParent());
Dom.XmlNode added = root.addChildElement('second', null, null);
added.setAttributeNs('id', 's', null, null);
System.assertEquals('s', added.getAttributeValue('id', null));
System.assertEquals(true, added.removeAttribute('id', null));
System.assertEquals(false, added.removeAttribute('id', null));
added.setNamespace('p', 'urn:p');
System.assertEquals('urn:p', added.getNamespaceFor('p'));
System.assertEquals('p', added.getPrefixFor('urn:p'));
System.assert(doc.toXmlString().contains('<child id="c">there</child>'));
System.assert(root.toXmlString().contains('<child id="c">there</child>'));
System.assertEquals(true, root.removeChild(added));
System.assertEquals(false, root.removeChild(added));
System.assertEquals(3, root.getChildren().size());
Dom.Document built = new Dom.Document();
Dom.XmlNode request = built.createRootElement('request', null, null);
request.setAttribute('xmlns', 'urn:test');
request.addChildElement('name', null, null).addTextNode('local');
Id accountId = '001000000000001AAA';
request.addChildElement('payer', null, null).addTextNode(accountId);
System.assertEquals('request', built.getRootElement().getName());
System.assert(built.toXmlString().contains('<request'));
System.assert(built.toXmlString().contains('<name>local</name>'));
System.assert(built.toXmlString().contains('<payer>001000000000001AAA</payer>'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringSearchIndexesAndBetween(t *testing.T) {
	program, err := CompileAnonymous(`
String text = 'café café';
System.assertEquals(3, text.indexOf('é'));
System.assertEquals(8, text.lastIndexOf('é'));
System.assertEquals(5, text.indexOf('c', 2));
System.assertEquals(0, text.indexOf('c', -10));
System.assertEquals(-1, text.indexOf('c', 9));
System.assertEquals(4, text.indexOf('', 4));
System.assertEquals(9, text.indexOf('', 99));
System.assertEquals(0, text.lastIndexOf('c', 4));
System.assertEquals(5, text.lastIndexOf('c', 99));
System.assertEquals(-1, text.lastIndexOf('c', -1));
System.assertEquals(4, text.lastIndexOf('', 4));
System.assertEquals(9, text.lastIndexOf('', 99));

String wrapped = 'prefix [value] suffix';
System.assertEquals('value', wrapped.substringBetween('[', ']'));
System.assertEquals('value', '--value--'.substringBetween('--'));
String missing = wrapped.substringBetween('<', '>');
System.assertEquals(null, missing);
System.assertEquals('', 'abc'.substringBetween(''));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRestContextRequestAndResponseShapes(t *testing.T) {
	program, err := CompileAnonymous(`
RestRequest req = new RestRequest();
req.requestURI = '/services/apexrest/widgets/42?expand=true';
req.resourcePath = '/widgets/42';
req.httpMethod = 'PATCH';
req.remoteAddress = '127.0.0.1';
req.requestBody = Blob.valueOf('{"name":"Acme"}');
req.addHeader('Content-Type', 'first');
req.addHeader('content-type', 'application/json');
req.addParameter('expand', 'true');
req.addParameter('sort', 'name');
RestContext.request = req;
req.headers = null;
	req.ADDHEADER('X-Rebuilt', 'yes');
	req.params = null;
	req.addPARAMETER('rebuilt', 'true');

System.assertEquals('/services/apexrest/widgets/42?expand=true', RestContext.request.requestURI);
System.assertEquals('/widgets/42', RestContext.request.resourcePath);
System.assertEquals('PATCH', RestContext.request.httpMethod);
System.assertEquals('127.0.0.1', RestContext.request.remoteAddress);
System.assertEquals('{"name":"Acme"}', RestContext.request.requestBody.toString());
	System.assertEquals('yes', RestContext.request.GETHEADER('x-rebuilt'));
	System.assertEquals(1, RestContext.request.GETHEADERKEYS().size());
	System.assert(RestContext.request.getHeaderKeys().contains('X-Rebuilt'));
	System.assertEquals('true', RestContext.request.GETPARAMETER('rebuilt'));
	System.assertEquals(1, RestContext.request.GETPARAMETERKEYS().size());
System.assert(RestContext.request.getParameterKeys().contains('rebuilt'));

RestContext.response.statusCode = 201;
RestContext.response.responseBody = Blob.valueOf('created');
	RestContext.response.ADDHEADER('Location', '/services/apexrest/widgets/41');
	RestContext.response.ADDHEADER('location', '/services/apexrest/widgets/42');
System.assertEquals(201, RestContext.response.statusCode);
System.assertEquals('created', RestContext.response.responseBody.toString());
System.assertEquals('/services/apexrest/widgets/42', RestContext.response.headers.get('location'));
	System.assertEquals('/services/apexrest/widgets/42', RestContext.response.GETHEADER('LOCATION'));
	System.assertEquals(1, RestContext.response.GETHEADERKEYS().size());
System.assert(RestContext.response.getHeaderKeys().contains('location'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRestContextStaticFieldsAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
RestRequest req = new RestRequest();
req.requestURI = '/services/apexrest/case';
req.requestBody = Blob.valueOf('body');
restcontext.REQUEST = req;
System.assertEquals('/services/apexrest/case', RESTCONTEXT.request.requestURI);
System.assertEquals('body', RestContext.Request.RequestBody.toString());

RestResponse res = new RestResponse();
res.statusCode = 202;
RESTCONTEXT.Response = res;
System.assertEquals(202, restcontext.RESPONSE.statusCode);
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestExecRestContextLifecycleEdges(t *testing.T) {
	program, err := CompileAnonymous(`
RestContext.request = null;
System.assertEquals(null, RestContext.request);
RestContext.response = null;
System.assertEquals(200, RestContext.response.statusCode);
RestContext.response.addHeader('X-Lifecycle', 'rebuilt');
System.assertEquals('rebuilt', RestContext.response.getHeader('x-lifecycle'));
RestResponse replacement = new RestResponse();
replacement.statusCode = 204;
RestContext.response = replacement;
System.assertEquals(204, RestContext.response.statusCode);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRestContextRejectsWrongStaticTypes(t *testing.T) {
	program, err := CompileAnonymous(`RestContext.request = new RestResponse();`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "RestContext.request expects RestRequest") {
		t.Fatalf("expected RestContext type error, got %v", err)
	}
}

func TestExecRestContextNestedNullDereference(t *testing.T) {
	program, err := CompileAnonymous(`
RestContext.request = null;
System.debug(RestContext.request.requestURI);
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "NullPointerException" {
		t.Fatalf("err = %#v, want NullPointerException", err)
	}
}

func TestExecPageReferenceRenderingUnsupported(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "content",
			src:  `PageReference page = new PageReference('/apex/Trail'); page.getContent();`,
			want: `unsupported call "PageReference.getContent local Visualforce page rendering surface"`,
		},
		{
			name: "pdf",
			src:  `PageReference page = new PageReference('/apex/Trail'); page.getContentAsPDF();`,
			want: `unsupported call "PageReference.getContentAsPDF local Visualforce page rendering surface"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want {
				t.Fatalf("err = %#v, want %s", err, tc.want)
			}
		})
	}
}

func TestExecPageReferenceURLStateAccessors(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/apex/Trail?id=001000000000001&mode=edit&mode=view');
System.assertEquals('/apex/Trail?id=001000000000001&mode=edit&mode=view', page.getUrl());
System.assertEquals(false, page.getRedirect());
page.setRedirect(true);
System.assertEquals(true, page.getRedirect());
System.assertEquals('001000000000001', page.getParameters().get('id'));
System.assertEquals('view', page.getParameters().get('mode'));
page.getHeaders().put('X-Local', 'yes');
page.setCookies(new List<Cookie>{new Cookie('sid', 'abc', '/', 60, true, 'Lax', true)});
System.assertEquals('001000000000001', page.getParameters().get('id'));
System.assertEquals('001000000000001', page.getparameters().get('id'));
System.assertEquals('yes', page.getHeaders().get('X-Local'));
Cookie sid = page.getCookies().get('sid');
System.assertEquals('sid', sid.getName());
System.assertEquals('abc', sid.getValue());
System.assertEquals('/', sid.getPath());
System.assertEquals(60, sid.getMaxAge());
System.assertEquals(true, sid.isSecure());
System.assertEquals('Lax', sid.getSameSite());
System.assertEquals(true, sid.isHttpOnly());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecLocationAndQueueableDuplicateSignatureValueObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Location left = Location.newInstance(37.7749, -122.4194);
Location right = Location.newInstance(34.0522, -118.2437);
Address address = new Address();
address.latitude = 37.7749;
address.longitude = -122.4194;
System.assertEquals(37.7749, left.getLatitude());
System.assertEquals(-122.4194, left.getLongitude());
System.assertEquals(37.7749, address.getLatitude());
System.assert(left.getDistance(right, 'mi') > 300);
System.assert(address.getDistance(right, 'mi') > 300);
System.assert(Location.getDistance(left, right, 'km') > 500);
QueueableDuplicateSignature sig = QueueableDuplicateSignature.builder()
	.addString('job')
	.addInteger(42)
	.addId('001000000000001AAA')
	.build();
System.assert(sig.toString().contains('String:job'));
System.assert(sig.toString().contains('Integer:42'));
System.assert(sig.toString().contains('Id:001000000000001AAA'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecProcessParameterValueObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Process.InputParameter input = new Process.InputParameter(
	'accountId',
	'Account id',
	Process.PluginDescribeResult.ParameterType.STRING,
	true
);
Process.OutputParameter output = new Process.OutputParameter(
	'status',
	'Status text',
	Process.PluginDescribeResult.ParameterType.BOOLEAN
);
System.assertEquals('accountId', input.name);
System.assertEquals('Account id', input.description);
System.assertEquals(Process.PluginDescribeResult.ParameterType.STRING, input.parameterType);
System.assertEquals(true, input.required);
System.assertEquals('status', output.name);
System.assertEquals('Status text', output.description);
System.assertEquals(Process.PluginDescribeResult.ParameterType.BOOLEAN, output.parameterType);
System.assertEquals('STRING', input.parameterType.name());
System.assertEquals(11, Process.PluginDescribeResult.ParameterType.values().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDomainValueObjects(t *testing.T) {
	program, err := CompileAnonymous(`
String orgHost = DomainCreator.getOrgMyDomainHostname();
String setupHost = DomainCreator.getSetupHostname();
String vfHost = DomainCreator.getVisualforceHostname('pkg');
System.assertEquals('oaer.my.salesforce.local', orgHost);
System.assertEquals('oaer.setup.local', setupHost);
System.assertEquals('pkg--oaer.visualforce.local', vfHost);
Domain orgDomain = DomainParser.parse(orgHost);
Domain vfDomain = DomainParser.parse('https://' + vfHost + '/apex/Home');
System.assertEquals('oaer', orgDomain.getMyDomainName());
System.assertEquals('', orgDomain.getPackageName());
System.assertEquals(null, orgDomain.getSandboxName());
System.assertEquals('pkg', vfDomain.getPackageName());
System.assertEquals('pkg--oaer.visualforce.local', vfDomain.toString());
System.assertEquals('oaer.my.salesforce.local', new Domain().toString());
Domain cloned = (Domain)vfDomain.clone();
System.assertEquals(vfDomain.toString(), cloned.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedDTOConstructorsAndFluentBuilders(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String,Object> context = new Map<String,Object>{'channel' => 'web'};
CommerceBuyGrp.BuyerGroupRequest request = new CommerceBuyGrp.BuyerGroupRequest('store-one', 'account-one', context);
System.assertEquals('store-one', request.getStoreId());
System.assertEquals('account-one', request.getAccountId());
System.assertEquals('web', (String)request.getRequestContextParameters().get('channel'));

CartExtension.ItemArrange arrange = new CartExtension.ItemArrange.Builder()
	.withCartItemId('0aB000000000001AAA')
	.withProductId('01t000000000001AAA')
	.withQuantity(3)
	.withDeliverToCity('Portland')
	.withDeliverToLatitude(45.5152)
	.withDeliverToLongitude(-122.6784)
	.build();
System.assertEquals('0aB000000000001AAA', (String)arrange.getCartItemId());
System.assertEquals('01t000000000001AAA', (String)arrange.getProductId());
System.assertEquals(3, arrange.getQuantity());
System.assertEquals('Portland', arrange.deliverToCity);
System.assertEquals(45.5152, arrange.deliverToLatitude);
System.assertEquals('Portland', arrange.getDeliveryAddress().getCity());
System.assertEquals(45.5152, arrange.getDeliveryAddress().getLatitude());

CartExtension.CartItemChange change = new CartExtension.CartItemChange.Builder()
	.withAdded(true)
	.withQuantityIncreased(true)
	.withRemoved(false)
	.build();
System.assertEquals(true, change.isAdded());
System.assertEquals(true, change.isQuantityIncreased());
System.assertEquals(false, change.isRemoved());

CartExtension.CouponChange couponChange = new CartExtension.CouponChange.Builder()
	.withAdded(true)
	.withRemoved(false)
	.build();
System.assertEquals(true, couponChange.isAdded());
System.assertEquals(false, couponChange.isRemoved());

CartExtension.CartDeliveryGroupChange deliveryChange = new CartExtension.CartDeliveryGroupChange.Builder()
	.withChangedDeliveryGroup(new CartExtension.OptionalCartDeliveryGroup())
	.build();
System.assertNotEquals(null, deliveryChange.getChangedDeliveryGroup());

CartExtension.BuyerActionDetails details = new CartExtension.BuyerActionDetails.Builder()
	.withCheckoutStarted(true)
	.withCartItemChanges(new List<CartExtension.CartItemChange>{change})
	.withCouponChanges(new List<CartExtension.CouponChange>{couponChange})
	.withDeliveryGroupChanges(new List<CartExtension.CartDeliveryGroupChange>{deliveryChange})
	.build();
System.assertEquals(true, details.isCheckoutStarted());
System.assertEquals(1, details.getCartItemChanges().size());
System.assertEquals(1, details.getCouponChanges().size());
System.assertEquals(1, details.getDeliveryGroupChanges().size());

CartExtension.Cart cart = new CartExtension.Cart();
CartExtension.ItemArrangementRequest arrangementRequest = new CartExtension.ItemArrangementRequest.Builder()
	.withCart(cart)
	.withItemArrangeList(new List<CartExtension.ItemArrange>{arrange})
	.build();
System.assertEquals(cart, arrangementRequest.getCart());
System.assertEquals(1, arrangementRequest.getItemArrangeList().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedConnectApiAndCommerceDTODataCarriers(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.ActionInputRepresentation action = new ConnectApi.ActionInputRepresentation();
action.actionType = 'Flow';
action.actionValue = 'SendEmail';
System.assertEquals('Flow', (String)action.actionType);
System.assertEquals('SendEmail', (String)action.actionValue);
System.assert(action.toString().contains('ActionInputRepresentation'));
ConnectApi.ActionInputRepresentation clonedAction = (ConnectApi.ActionInputRepresentation)action.clone();
System.assertEquals('Flow', (String)clonedAction.actionType);

ConnectApi.AccountSyncToExternalInputRepresentation sync = new ConnectApi.AccountSyncToExternalInputRepresentation();
sync.adServerAccountId = 'external-account';
sync.contactIds = new List<String>{'003000000000001AAA'};
sync.customFields = new Map<String,Object>{'region' => 'NA'};
System.assertEquals('external-account', (String)sync.adServerAccountId);
System.assertEquals(1, ((List<String>)sync.contactIds).size());
System.assertEquals('NA', (String)((Map<String,Object>)sync.customFields).get('region'));

CommerceExtension.Resolution registered = new CommerceExtension.Resolution(CommerceExtension.ResolutionStates.EXECUTE_REGISTERED);
CommerceExtension.Resolution provider = new CommerceExtension.Resolution('local-provider');
System.assertEquals(CommerceExtension.ResolutionStates.EXECUTE_REGISTERED, registered.getResolutionState());
System.assertEquals('local-provider', provider.getProviderName());
System.assertEquals(3, CommerceExtension.ResolutionStates.values().size());
System.assertEquals(CommerceExtension.ResolutionStates.OFF, CommerceExtension.ResolutionStates.valueOf('OFF'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedConnectApiServiceCallsRemainUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.FeedElementPage page = ConnectApi.ChatterFeeds.getFeedElementsFromFeed(null, null);
System.assertEquals(null, page);
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err == nil {
		t.Fatal("expected generated ConnectApi service call to be unsupported")
	}
	if !strings.Contains(err.Error(), `unsupported call "ConnectApi.ChatterFeeds.getFeedElementsFromFeed"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecDataWeaveScriptResultCarriers(t *testing.T) {
	program, err := CompileAnonymous(`
DataWeave.Script script = DataWeave.Script.createScript('helloWorld');
DataWeave.Result result = script.execute(new Map<String,Object>());
System.assertEquals('"Hello World"', result.getValueAsString());
System.assertEquals('text/plain', result.getMimeType());
System.assertEquals('"Hello World"', (String)result.valueAsString);

Map<String,Object> inputs = new Map<String,Object>{'records' => new List<String>{'a', 'b'}};
DataWeave.Result projected = DataWeave.Script.createScript('records').execute(inputs);
System.assertEquals(2, ((List<String>)projected.getValue()).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDataWeaveScriptErrorThrowsScriptException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	DataWeave.Script.createScript('error').execute(new Map<String,Object>());
	System.assert(false, 'expected DataWeaveScriptException');
} catch (Exception ex) {
	Assert.isInstanceOfType(ex, DataWeaveScriptException.class);
	System.assert(ex.getMessage().startsWith('Division by zero'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDataWeaveExcelOutputErrorThrowsScriptException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	DataWeave.Script.createScript('excelOutputError').execute(new Map<String,Object>{'records' => new List<Contact>{new Contact(FirstName = 'John', LastName = 'Doe')}});
	System.assert(false, 'expected DataWeaveScriptException');
} catch (Exception ex) {
	Assert.isInstanceOfType(ex, DataWeaveScriptException.class);
	System.assert(ex.getMessage().contains('application/xlsx'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDataWeaveMultipleInputsReturnsXMLString(t *testing.T) {
	program, err := CompileAnonymous(`
String products = '[ { "type": "book", "price": 30, "properties": { "title": "Everyday Italian", "author": [ "Giada De Laurentiis" ], "year": 2005 } } ]';
String attributes = '{ "publishedAfter": 2004 }';
String exchangeRates = '{ "USD": [ {"currency": "EUR", "ratio":0.92}, {"currency": "ARS", "ratio":8.76} ]}';
String output = DataWeave.Script.createScript('multipleInputs')
	.execute(new Map<String,Object>{'products' => products, 'attributes' => attributes, 'exchangeRates' => exchangeRates})
	.getValueAsString();
System.assert(output.contains('<author>Giada De Laurentiis</author>'));
System.assert(output.contains('<price currency="ARS">262.8</price>'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDataWeaveJsonDateFormatPreservesRecipeFieldOrder(t *testing.T) {
	program, err := CompileAnonymous(`
Contact contact = new Contact(FirstName = 'John', LastName = 'Doe');
contact.CreatedDate = Datetime.newInstanceGMT(2026, 5, 2, 12, 0, 0);
String jsonText = DataWeave.Script.createScript('jsonDateFormat')
	.execute(new Map<String,Object>{'records' => new List<Contact>{contact}})
	.getValueAsString();
String expected =
	'{\n' +
	'  "users": [\n' +
	'    {\n' +
	'      "firstName": "John",\n' +
	'      "lastName": "Doe",\n' +
	'      "createdDate": "12:00:00 PM, May 02, 2026"\n' +
	'    }\n' +
	'  ]\n' +
	'}';
System.assertEquals(expected, jsonText);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDataWeaveRecipeConversionsReturnStructuredValues(t *testing.T) {
	program, err := CompileAnonymous(`
String csv = 'FirstName,LastName,Email\nAda,Lovelace,ada@example.test\nGrace,Hopper,grace@example.test';
DataWeave.Result contactsResult = DataWeave.Script.createScript('csvToContacts').execute(new Map<String,Object>{'records' => csv});
List<Contact> contacts = (List<Contact>)contactsResult.getValue();
System.assertEquals(2, contacts.size());
System.assertEquals('Ada', contacts.get(0).FirstName);
System.assertEquals('Hopper', contacts.get(1).LastName);

String jsonText = DataWeave.Script.createScript('csvToJsonBasic').execute(new Map<String,Object>{'payload' => csv}).getValueAsString();
System.assert(jsonText.contains('"FirstName": "Ada",'));
List<Object> jsonList = (List<Object>)JSON.deserializeUntyped(jsonText);
System.assertEquals(2, jsonList.size());
Map<String,Object> first = (Map<String,Object>)jsonList.get(0);
System.assertEquals('Ada', first.get('FirstName'));

String snake = 'first_name,last_name,email\nAbel,Maclead,a.m@demo.org';
List<Contact> snakeContacts = (List<Contact>)DataWeave.Script.createScript('csvToContacts').execute(new Map<String,Object>{'records' => snake}).getValue();
System.assertEquals('Abel', snakeContacts.get(0).FirstName);
List<Contact> jsonContacts = (List<Contact>)DataWeave.Script.createScript('jsonToContacts').execute(new Map<String,Object>{'records' => '[{"first_name":"Abel","last_name":"Maclead","email":"a.m@demo.org"}]'}).getValue();
System.assertEquals('Maclead', jsonContacts.get(0).LastName);

String renamedJSON = DataWeave.Script.createScript('csvToJsonWithFieldRenaming').execute(new Map<String,Object>{'payload' => 'first_name,last_name,company,address\nAbel,Maclead,Acme,Street'}).getValueAsString();
List<Object> renamedList = (List<Object>)JSON.deserializeUntyped(renamedJSON);
Map<String,Object> renamed = (Map<String,Object>)renamedList.get(0);
System.assertEquals('Abel', renamed.get('FirstName'));
System.assertEquals('Street', renamed.get('MailingStreet'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringStdlibMoreMethods(t *testing.T) {
	program, err := CompileAnonymous(`
String letters = 'a b c 5 xyz';
System.assertEquals('1 1 1 5 111', letters.replaceAll('[a-zA-Z]', '1'));
System.assertEquals('aXc abc', 'abc abc'.replaceFirst('b(?=c)', 'X'));
System.assertEquals('aXc aXc', 'abc abc'.replaceAll('b(?=c)', 'X'));
System.assertEquals('Id, , Name FROM Account',
	'Id, (SELECT Id FROM Contacts), Name FROM Account'.replaceAll('(?i)(?s)\\(\\s*SELECT\\s.*?\\)(?=\\s*,|\\s*FROM\\s|\\s*$)', ''));
String lettersFirst = 'a b c 11 xyz';
System.assertEquals('a b c 11 2z', lettersFirst.replaceFirst('[a-zA-Z]{2}', '2'));
String splitSource = 'boo:and:moo';
List<String> limitTwo = splitSource.split(':', 2);
System.assertEquals(2, limitTwo.size());
System.assertEquals('boo', limitTwo.get(0));
System.assertEquals('and:moo', limitTwo.get(1));
List<String> limitFive = splitSource.split('o', 5);
System.assertEquals(5, limitFive.size());
System.assertEquals('b', limitFive.get(0));
System.assertEquals('', limitFive.get(1));
System.assertEquals(':and:m', limitFive.get(2));
System.assertEquals('', limitFive.get(3));
System.assertEquals('', limitFive.get(4));
List<String> limitZero = splitSource.split('o', 0);
System.assertEquals(3, limitZero.size());
System.assertEquals(':and:m', limitZero.get(2));
List<String> limitNegative = splitSource.split('o', -2);
System.assertEquals(5, limitNegative.size());
System.assertEquals('', limitNegative.get(4));
String helloJaneSpace = 'Hello Jane';
System.assert(helloJaneSpace.containsWhitespace());
String helloJane = 'HelloJane';
System.assert(!helloJane.containsWhitespace());
String helloHello = 'Hello Hello';
System.assertEquals(2, helloHello.countMatches('Hello'));
String aaa = 'aaa';
System.assertEquals(0, aaa.countMatches(''));
String hello = 'hello';
System.assert(hello.containsAny('hx'));
System.assert(!hello.containsAny('xz'));
Id accountId = Id.valueOf('001000000000001');
System.assert('prefix-001000000000001'.containsIgnoreCase(accountId));
String abcde = 'abcde';
System.assert(abcde.containsNone('fg'));
System.assert(!abcde.containsNone('df'));
String abba = 'abba';
System.assert(abba.containsOnly('abcd'));
String abbaXyz = 'abba xyz';
System.assert(!abbaXyz.containsOnly('abcd'));
String oneSpace = ' ';
System.assert(oneSpace.isWhitespace());
String empty = '';
System.assert(empty.isWhitespace());
String sil80 = 'SIL80';
System.assert(!sil80.isWhitespace());
String alphaAccent = 'abcÉ';
System.assert(alphaAccent.isAlpha());
String alphaDigits = 'abc 21';
System.assert(!alphaDigits.isAlpha());
String alphaSpace = 'aA Bb';
System.assert(alphaSpace.isAlphaSpace());
String alphaDollar = 'aA$Bb';
System.assert(!alphaDollar.isAlphaSpace());
String abc021 = 'abc021';
System.assert(abc021.isAlphanumeric());
String romanNumeral = 'Ⅻ';
System.assert(!romanNumeral.isAlphanumeric());
String ae86 = 'AE 86';
System.assert(ae86.isAlphanumericSpace());
String alphaDollarDigits = 'aA$12';
System.assert(!alphaDollarDigits.isAlphanumericSpace());
String digits = '1234567890';
System.assert(digits.isNumeric());
String decimalPoint = '1.2';
System.assert(!decimalPoint.isNumeric());
String numericSpace = '1 2 3';
System.assert(numericSpace.isNumericSpace());
	String mixedCars = 'FD3S FC3S';
	System.assert(!mixedCars.isNumericSpace());
	System.assert(abcde.isAllLowerCase());
String lowerWithDigits = 'abc 123!';
System.assert(lowerWithDigits.isAllLowerCase());
	String abcDe = 'abcDe';
	System.assert(!abcDe.isAllLowerCase());
System.assert(!digits.isAllLowerCase());
System.assert(!empty.isAllLowerCase());
	String ABCDE = 'ABCDE';
	System.assert(ABCDE.isAllUpperCase());
String upperWithDigits = 'ABC 123!';
System.assert(upperWithDigits.isAllUpperCase());
	String ABCdE = 'ABCdE';
	System.assert(!ABCdE.isAllUpperCase());
System.assert(!digits.isAllUpperCase());
System.assert(!empty.isAllUpperCase());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringCSVFieldEscapingEdges(t *testing.T) {
	program, err := CompileAnonymous(`
String plain = 'plain';
System.assertEquals('plain', plain.escapeCsv());
System.assertEquals('plain', plain.unescapeCsv());
String comma = 'left,right';
System.assertEquals('"left,right"', comma.escapeCsv());
String quoted = 'He said "hi"';
System.assertEquals('"He said ""hi"""', quoted.escapeCsv());
System.assertEquals(quoted, quoted.escapeCsv().unescapeCsv());
System.assertEquals('a"b', '"a""b"'.unescapeCsv());
List<Integer> crlfCodes = new List<Integer>();
crlfCodes.add(13);
crlfCodes.add(10);
String crlf = String.fromCharArray(crlfCodes);
String lineBreak = 'line1' + crlf + 'line2';
System.assertEquals('"' + lineBreak + '"', lineBreak.escapeCsv());
System.assertEquals(lineBreak, lineBreak.escapeCsv().unescapeCsv());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringStdlibMoreRejectsBadRegex(t *testing.T) {
	program, err := CompileAnonymous(`String abc = 'abc';
abc.replaceAll('[', 'x');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestExecStringStdlibCompletionMethods(t *testing.T) {
	program, err := CompileAnonymous(`
String csv = 'a,"b",c';
System.assertEquals('"a,""b"",c"', csv.escapeCsv());
String escapedCsv = csv.escapeCsv();
System.assertEquals(csv, escapedCsv.unescapeCsv());
String html = '<tag attr=''x''>&';
System.assertEquals('&lt;tag attr=&#39;x&#39;&gt;&amp;', html.escapeHtml4());
String escapedHtml = html.escapeHtml4();
System.assertEquals(html, escapedHtml.unescapeHtml4());
System.assertEquals('&lt;tag attr=&apos;x&apos;&gt;&amp;', html.escapeXml());
String escapedXml = html.escapeXml();
System.assertEquals(html, escapedXml.unescapeXml());
String slash = 'a/b';
System.assertEquals('a\/b', slash.escapeEcmaScript());
String escapedSlash = slash.escapeEcmaScript();
System.assertEquals(slash, escapedSlash.unescapeEcmaScript());
String quoted = 'He said "hi"';
System.assertEquals('He said \"hi\"', quoted.escapeJava());
String escapedQuoted = quoted.escapeJava();
System.assertEquals(quoted, escapedQuoted.unescapeJava());
String omega = 'AΩ';
System.assertEquals('A\u03A9', omega.escapeUnicode());
String escapedOmega = omega.escapeUnicode();
System.assertEquals(omega, escapedOmega.unescapeUnicode());
System.assertEquals('Bob\''s', String.escapeSingleQuotes('Bob''s'));
List<String> formatArgs = new List<String>();
formatArgs.add('Ada');
formatArgs.add('Lovelace');
System.assertEquals('Hello Ada Lovelace', String.format('Hello {0} {1}', formatArgs));
System.assertEquals('Use {0} then Ada', String.format('Use ''{0}'' then {0}', formatArgs));
System.assertEquals('Lovelace/Ada/Lovelace/{2}', String.format('{1}/{0}/{1}/{2}', formatArgs));
String alphabet = 'abcdefghijklmnopqrstuvwxyz';
System.assertEquals('abcdefg...', alphabet.abbreviate(10));
System.assertEquals('...ijklmn...', alphabet.abbreviate(8, 12));
String machine = 'i am a machine';
System.assertEquals('robot', machine.difference('i am a robot'));
String interstate = 'interstate';
System.assertEquals('interst', interstate.commonPrefix('interstellar'));
List<String> prefixes = new List<String>();
prefixes.add('flower');
prefixes.add('flow');
prefixes.add('flight');
System.assertEquals('fl', String.getCommonPrefix(prefixes));
String kitten = 'kitten';
System.assertEquals(3, kitten.getLevenshteinDistance('sitting'));
System.assertEquals(3, String.getLevenshteinDistance('kitten', 'sitting'));
String chars = 'AΩ';
System.assertEquals(65, chars.charAt(0));
System.assertEquals(937, chars.codePointAt(1));
System.assertEquals(65, chars.codePointBefore(1));
System.assertEquals(2, chars.codePointCount(0, 2));
List<Integer> charCodes = chars.getChars();
System.assertEquals(2, charCodes.size());
System.assertEquals(65, charCodes.get(0));
System.assertEquals(937, charCodes.get(1));
System.assertEquals(chars, String.fromCharArray(charCodes));
String printable = 'AZ 19~';
String nonPrintable = 'Snow Ω';
System.assert(printable.isAsciiPrintable());
System.assert(!nonPrintable.isAsciiPrintable());
String typeSource = 'ab12 CD';
List<String> splitType = typeSource.splitByCharacterType();
System.assertEquals(4, splitType.size());
System.assertEquals('ab', splitType.get(0));
System.assertEquals('12', splitType.get(1));
System.assertEquals(' ', splitType.get(2));
System.assertEquals('CD', splitType.get(3));
String camelSource = 'HTTPServer42';
List<String> camel = camelSource.splitByCharacterTypeCamelCase();
System.assertEquals(3, camel.size());
System.assertEquals('HTTP', camel.get(0));
System.assertEquals('Server', camel.get(1));
System.assertEquals('42', camel.get(2));
String edge = 'abΩcdΩef';
System.assertEquals(1, edge.indexOfAny('bΩ'));
System.assertEquals(2, edge.indexOfAny('Ω'));
System.assertEquals(7, edge.lastIndexOfAny('Ωf'));
System.assertEquals(4, edge.indexOfAnyBut('abΩc'));
String repeated = 'one fish two fish red fish';
System.assertEquals(13, repeated.ordinalIndexOf('fish', 2));
System.assertEquals(9, repeated.lastOrdinalIndexOf('two', 1));
System.assertEquals(-1, repeated.ordinalIndexOf('fish', 0));
String overlaySource = 'abcdef';
System.assertEquals('abZZef', overlaySource.overlay('ZZ', 2, 4));
System.assertEquals('XXabcdef', overlaySource.overlay('XX', -2, 0));
System.assertEquals('cdefab', overlaySource.rotate(-2));
System.assertEquals('efabcd', overlaySource.rotate(2));
String mixed = 'The Ω42';
System.assertEquals('tHE ω42', mixed.swapCase());
String stripSource = '  abc  ';
System.assertEquals('abc', stripSource.strip());
System.assertEquals('abc  ', stripSource.stripStart());
System.assertEquals('  abc', stripSource.stripEnd());
String blankStrip = '   ';
System.assertEquals(null, blankStrip.stripToNull());
System.assertEquals('', blankStrip.stripToEmpty());
String yxy = 'xyabczy';
System.assertEquals('abc', yxy.strip('xyz'));
List<String> stripItems = new List<String>();
stripItems.add('  one  ');
stripItems.add('  two');
List<String> strippedItems = String.stripAll(stripItems);
System.assertEquals('one', strippedItems.get(0));
System.assertEquals('two', strippedItems.get(1));
String caseSource = 'Force FORCE force';
System.assertEquals('Force force', caseSource.replaceOnce('FORCE ', ''));
System.assertEquals('Cloud Cloud Cloud', caseSource.replaceIgnoreCase('force', 'Cloud'));
System.assertEquals('  ', caseSource.removeIgnoreCase('force'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringStdlibEntityAndNullEdges(t *testing.T) {
	program, err := CompileAnonymous(`
String htmlCore = '"''<>&';
System.assertEquals('&quot;&#39;&lt;&gt;&amp;', htmlCore.escapeHtml4());
System.assertEquals('&quot;&#39;&lt;&gt;&amp;', htmlCore.escapeHtml3());
String escapedHtmlCore = '&quot;&lt;&gt;&amp;';
System.assertEquals('"<>&', escapedHtmlCore.unescapeHtml4());
String escapedHtmlDecimal = '&#39;';
String htmlDecimal = escapedHtmlDecimal.unescapeHtml4();
System.assertEquals(39, htmlDecimal.codePointAt(0));
String escapedHtmlHex = '&#x27;';
String htmlHex = escapedHtmlHex.unescapeHtml4();
System.assertEquals(39, htmlHex.codePointAt(0));
String namedHtml = '&nbsp;&copy;&reg;&trade;&euro;&mdash;&ndash;&hellip;&bull;&ldquo;&rdquo;&lsquo;&rsquo;&cent;&pound;&yen;&sect;&para;&middot;';
String unescapedNamedHtml = namedHtml.unescapeHtml4();
System.assertEquals(19, unescapedNamedHtml.length());
System.assertEquals(160, unescapedNamedHtml.codePointAt(0));
System.assertEquals(169, unescapedNamedHtml.codePointAt(1));
System.assertEquals(174, unescapedNamedHtml.codePointAt(2));
System.assertEquals(8482, unescapedNamedHtml.codePointAt(3));
System.assertEquals(8364, unescapedNamedHtml.codePointAt(4));
System.assertEquals(8212, unescapedNamedHtml.codePointAt(5));
System.assertEquals(8211, unescapedNamedHtml.codePointAt(6));
System.assertEquals(8230, unescapedNamedHtml.codePointAt(7));
System.assertEquals(8226, unescapedNamedHtml.codePointAt(8));
System.assertEquals(8220, unescapedNamedHtml.codePointAt(9));
System.assertEquals(8221, unescapedNamedHtml.codePointAt(10));
System.assertEquals(8216, unescapedNamedHtml.codePointAt(11));
System.assertEquals(8217, unescapedNamedHtml.codePointAt(12));
System.assertEquals(162, unescapedNamedHtml.codePointAt(13));
System.assertEquals(163, unescapedNamedHtml.codePointAt(14));
System.assertEquals(165, unescapedNamedHtml.codePointAt(15));
System.assertEquals(167, unescapedNamedHtml.codePointAt(16));
System.assertEquals(182, unescapedNamedHtml.codePointAt(17));
System.assertEquals(183, unescapedNamedHtml.codePointAt(18));
String unescapedNamedHtml3 = '&copy;&reg;'.unescapeHtml3();
System.assertEquals(169, unescapedNamedHtml3.codePointAt(0));
System.assertEquals(174, unescapedNamedHtml3.codePointAt(1));
String unknownHtml = '&notarealentity;&apos;';
System.assertEquals('&notarealentity;&apos;', unknownHtml.unescapeHtml4());
System.assertEquals('&notarealentity;&apos;', unknownHtml.unescapeHtml3());
System.assertEquals(namedHtml, namedHtml.unescapeXml());
System.assertEquals('&quot;&apos;&lt;&gt;&amp;', htmlCore.escapeXml());
System.assertEquals('&quot;&apos;&lt;&gt;&amp;', htmlCore.escapeXml10());
System.assertEquals('&quot;&apos;&lt;&gt;&amp;', htmlCore.escapeXml11());
System.assertEquals('"<>&', escapedHtmlCore.unescapeXml());
String escapedXmlApos = '&apos;';
String xmlApos = escapedXmlApos.unescapeXml();
System.assertEquals(39, xmlApos.codePointAt(0));
String escapedXmlNumeric = '&#65;&#x41;';
System.assertEquals('AA', escapedXmlNumeric.unescapeXml10());
System.assertEquals('&#31;', '&#31;'.unescapeXml10());
String xml11Restricted = '&#31;'.unescapeXml11();
System.assertEquals(31, xml11Restricted.codePointAt(0));
String xml10AllowedWhitespace = '&#9;&#10;&#13;';
System.assertEquals(3, xml10AllowedWhitespace.unescapeXml10().length());
String xml11LowControl = '&#1;';
System.assertEquals(1, xml11LowControl.unescapeXml11().codePointAt(0));
String replacementEntity = '&#xFFFD;';
String replacementValue = replacementEntity.unescapeXml();
System.assertEquals(65533, replacementValue.codePointAt(0));
String malformedXml = '&#xZZ;&copy;';
System.assertEquals('&#xZZ;&copy;', malformedXml.unescapeXml11());
String invalidXmlNumeric = '&#0;&#x0;&#xD800;&#55296;&#x110000;&#+65;&#x+41;';
System.assertEquals(invalidXmlNumeric, invalidXmlNumeric.unescapeXml());
System.assertEquals('AZ', '&#65;&#x5a;'.unescapeXml11());
String replaceEmpty = 'abc';
System.assertEquals('abc', replaceEmpty.replace('', 'x'));
System.assertEquals('abc', replaceEmpty.replaceOnce('', 'x'));
System.assertEquals('abc', replaceEmpty.replaceIgnoreCase('', 'x'));
System.assertEquals('abc', replaceEmpty.remove(''));
System.assert(String.isBlank(null));
System.assert(!String.isNotBlank(null));
System.assertEquals('', String.escapeSingleQuotes(''));
System.assertEquals('001000000000001AAA', String.escapeSingleQuotes((Id)'001000000000001AAA'));
System.assertEquals(null, String.escapeSingleQuotes(null));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringStdlibXMLVersionEscapes(t *testing.T) {
	program, err := CompileAnonymous(`
String xml10Invalid = String.fromCharArray(new List<Integer>{65, 1, 66, 0, 67});
System.assertEquals('ABC', xml10Invalid.escapeXml10());
String xml10Control = String.fromCharArray(new List<Integer>{65, 127, 66, 133, 67});
String xml10Escaped = xml10Control.escapeXml10();
System.assertEquals(65, xml10Escaped.codePointAt(0));
System.assertEquals(38, xml10Escaped.charAt(1));
System.assertEquals(35, xml10Escaped.charAt(2));
System.assertEquals(66, xml10Escaped.charAt(7));
System.assertEquals(133, xml10Escaped.codePointAt(8));
String xml11Control = String.fromCharArray(new List<Integer>{65, 0, 1, 66, 31, 67, 133});
String xml11Escaped = xml11Control.escapeXml11();
System.assertEquals(65, xml11Escaped.codePointAt(0));
System.assertEquals(38, xml11Escaped.charAt(1));
System.assertEquals(35, xml11Escaped.charAt(2));
System.assertEquals(66, xml11Escaped.charAt(5));
System.assertEquals(38, xml11Escaped.charAt(6));
System.assertEquals(35, xml11Escaped.charAt(7));
System.assertEquals(67, xml11Escaped.charAt(11));
System.assertEquals(133, xml11Escaped.codePointAt(12));
String xmlMarkup = '<a attr=''q''>&"';
System.assertEquals('&lt;a attr=&apos;q&apos;&gt;&amp;&quot;', xmlMarkup.escapeXml10());
System.assertEquals('&lt;a attr=&apos;q&apos;&gt;&amp;&quot;', xmlMarkup.escapeXml11());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringEntityAndLevenshteinFamilyCloseout(t *testing.T) {
	program, err := CompileAnonymous(`
String named = '&Alpha;&beta;&Omega;&spades;&loz;&rArr;&sum;';
String unescaped = named.unescapeHtml4();
System.assertEquals(7, unescaped.length());
System.assertEquals(913, unescaped.codePointAt(0));
System.assertEquals(946, unescaped.codePointAt(1));
System.assertEquals(937, unescaped.codePointAt(2));
System.assertEquals(9824, unescaped.codePointAt(3));
System.assertEquals(9674, unescaped.codePointAt(4));
System.assertEquals(8658, unescaped.codePointAt(5));
System.assertEquals(8721, unescaped.codePointAt(6));
System.assertEquals('&notarealentity;&apos;', '&notarealentity;&apos;'.unescapeHtml4());
String gumbo = 'gumbo';
System.assertEquals(2, gumbo.getLevenshteinDistance('gambol'));
System.assertEquals(2, gumbo.getLevenshteinDistance('gambol', 2));
System.assertEquals(-1, gumbo.getLevenshteinDistance('gambol', 1));
System.assertEquals(3, String.getLevenshteinDistance('kitten', 'sitting', 3));
System.assertEquals(-1, String.getLevenshteinDistance('kitten', 'sitting', 2));
System.assertEquals(1, 'café'.getLevenshteinDistance('cafe', 1));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringFinalFamilyEdges(t *testing.T) {
	program, err := CompileAnonymous(`
String raw = String.fromCharArray(new List<Integer>{8, 9, 10, 12, 13, 34, 39, 47, 92, 937, 128512});
System.assertEquals(raw, raw.escapeJava().unescapeJava());
System.assertEquals(raw, raw.escapeEcmaScript().unescapeEcmaScript());
String unicodeRaw = String.fromCharArray(new List<Integer>{8, 9, 10, 12, 13, 34, 39, 47, 937, 128512});
System.assertEquals(unicodeRaw, unicodeRaw.escapeUnicode().unescapeUnicode());
System.assertEquals('/', '/'.escapeJava());
System.assertEquals('\/', '/'.escapeEcmaScript());

String face = String.fromCharArray(new List<Integer>{128512});
System.assertEquals(1, face.length());
System.assertEquals(128512, face.codePointAt(0));
System.assertEquals(128512, face.codePointBefore(1));
System.assertEquals(1, face.codePointCount(0, 1));
List<Integer> faceChars = face.getChars();
System.assertEquals(1, faceChars.size());
System.assertEquals(128512, faceChars.get(0));

System.assertEquals('abc...', 'abcdefg'.abbreviate(6));
System.assertEquals('...mnopq...', 'abcdefghijklmnopqrstuvwxyz'.abbreviate(12, 11));
System.assertEquals('abXXef', 'abcdef'.overlay('XX', 4, 2));
System.assertEquals('abcdefZZ', 'abcdef'.overlay('ZZ', 99, 100));
System.assertEquals('aZ 9ω', 'Az 9Ω'.swapCase());

String xml10Boundary = String.fromCharArray(new List<Integer>{9, 10, 13, 31, 32, 55295, 57344, 65533, 128512});
String xml10Escaped = xml10Boundary.escapeXml10();
System.assertEquals(8, xml10Escaped.length());
System.assertEquals(9, xml10Escaped.codePointAt(0));
System.assertEquals(32, xml10Escaped.codePointAt(3));
System.assertEquals(55295, xml10Escaped.codePointAt(4));
System.assertEquals(128512, xml10Escaped.codePointAt(7));
String xml11Boundary = String.fromCharArray(new List<Integer>{0, 1, 8, 9, 10, 13, 31, 32});
String xml11Escaped = xml11Boundary.escapeXml11();
System.assertEquals(0, xml11Escaped.indexOf('&#1;'));
System.assertEquals(4, xml11Escaped.indexOf('&#8;'));
String tabChar = String.fromCharArray(new List<Integer>{9});
System.assertEquals(8, xml11Escaped.indexOf(tabChar));
System.assertEquals(11, xml11Escaped.indexOf('&#31;'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStringStdlibCompletionRejectsBadArguments(t *testing.T) {
	tests := []struct {
		method string
		args   []Value
	}{
		{method: "escapeCsv", args: []Value{String("x")}},
		{method: "abbreviate", args: []Value{Int(3)}},
		{method: "charAt", args: []Value{Int(-1)}},
		{method: "codePointAt", args: []Value{Int(9)}},
		{method: "codePointBefore", args: []Value{Int(0)}},
		{method: "codePointCount", args: []Value{Int(1), Int(0)}},
		{method: "splitByCharacterType", args: []Value{String("x")}},
		{method: "overlay", args: []Value{String("x"), Int(0)}},
		{method: "rotate", args: []Value{String("1")}},
		{method: "swapCase", args: []Value{String("x")}},
		{method: "strip", args: []Value{Int(1)}},
		{method: "stripToNull", args: []Value{String("x")}},
		{method: "stripToEmpty", args: []Value{String("x")}},
		{method: "ordinalIndexOf", args: []Value{String("a")}},
		{method: "replace", args: []Value{Null, String("x")}},
		{method: "replaceOnce", args: []Value{String("a"), Null}},
		{method: "remove", args: []Value{Null}},
		{method: "escapeHtml4", args: []Value{Null}},
		{method: "unescapeXml", args: []Value{Null}},
		{method: "getLevenshteinDistance", args: []Value{String("x"), Int(-1)}},
	}
	for _, tc := range tests {
		if _, handled, err := callStringMember(String("abc"), tc.method, tc.args); !handled || err == nil {
			t.Fatalf("%s expected handled error, handled=%v err=%v", tc.method, handled, err)
		}
	}
	if _, err := stringStatic("String.format", []Value{String("{0}"), String("x")}); err == nil {
		t.Fatal("String.format expected bad argument error")
	}
	if _, err := stringStatic("String.format", []Value{String("{0,number,#.00}"), List(Int(42))}); err == nil || !strings.Contains(err.Error(), "MessageFormat typed format elements") {
		t.Fatalf("String.format expected typed format unsupported error, got %v", err)
	}
	if _, err := stringStatic("String.format", []Value{String("{0"), List(Int(42))}); err == nil || !strings.Contains(err.Error(), "unmatched") {
		t.Fatalf("String.format expected unmatched brace error, got %v", err)
	}
	if _, err := stringStatic("String.fromCharArray", []Value{List(String("x"))}); err == nil {
		t.Fatal("String.fromCharArray expected bad argument error")
	}
	if _, err := stringStatic("String.stripAll", []Value{List(Int(1))}); err == nil {
		t.Fatal("String.stripAll expected bad argument error")
	}
	if _, err := stringStatic("String.getLevenshteinDistance", []Value{String("a"), String("b"), Int(-1)}); err == nil {
		t.Fatal("String.getLevenshteinDistance expected bad threshold error")
	}
	if _, _, err := callStringMember(String(`\u00ZZ`), "unescapeUnicode", nil); err == nil {
		t.Fatal("String.unescapeUnicode expected bad escape error")
	}
}

func TestStringStdlibMoreRejectsBadArgumentShapes(t *testing.T) {
	tests := []struct {
		method string
		args   []Value
	}{
		{method: "replaceAll", args: []Value{String("[a]"), Int(1)}},
		{method: "replaceFirst", args: []Value{String("[a]")}},
		{method: "split", args: []Value{String(","), String("2")}},
		{method: "containsWhitespace", args: []Value{String("x")}},
		{method: "isAlpha", args: []Value{String("x")}},
	}
	for _, tc := range tests {
		if _, handled, err := callStringMember(String("abc"), tc.method, tc.args); !handled || err == nil {
			t.Fatalf("%s expected handled error, handled=%v err=%v", tc.method, handled, err)
		}
	}
}

func TestStringRegexReplacementSplitAndUnsupportedEdges(t *testing.T) {
	replacedAll, handled, err := callStringMember(String("A1 B22 C333"), "replaceAll", []Value{String("([A-Z]+)([0-9]+)"), String("$10")})
	if err != nil || !handled || replacedAll.Text != "A0 B0 C0" {
		t.Fatalf("replaceAll = %#v handled=%v err=%v", replacedAll, handled, err)
	}
	replacedFirst, handled, err := callStringMember(String("A1 B22"), "replaceFirst", []Value{String("([A-Z]+)([0-9]+)"), String(`\$1`)})
	if err != nil || !handled || replacedFirst.Text != "$1 B22" {
		t.Fatalf("replaceFirst = %#v handled=%v err=%v", replacedFirst, handled, err)
	}
	unescapedWildcards, handled, err := callStringMember(String(`\Qname_%\E`), "replaceAll", []Value{String(`(?<!\\)_`), String(`\\E.\\Q`)})
	if err != nil || !handled || unescapedWildcards.Text != `\Qname\E.\Q%\E` {
		t.Fatalf("replaceAll negative lookbehind _ = %#v handled=%v err=%v", unescapedWildcards, handled, err)
	}
	unescapedWildcards, handled, err = callStringMember(String(`\Qname\E.\Q%\E`), "replaceAll", []Value{String(`(?<!\\)%`), String(`\\E.*\\Q`)})
	if err != nil || !handled || unescapedWildcards.Text != `\Qname\E.\Q\E.*\Q\E` {
		t.Fatalf("replaceAll negative lookbehind %% = %#v handled=%v err=%v", unescapedWildcards, handled, err)
	}
	escapedWildcard, handled, err := callStringMember(String(`\Qname\_%\E`), "replaceAll", []Value{String(`(?<!\\)_`), String(`\\E.\\Q`)})
	if err != nil || !handled || escapedWildcard.Text != `\Qname\_%\E` {
		t.Fatalf("replaceAll escaped negative lookbehind = %#v handled=%v err=%v", escapedWildcard, handled, err)
	}
	split, handled, err := callStringMember(String(":boo:"), "split", []Value{String(":"), Int(-1)})
	if err != nil || !handled || split.Kind != ValueList || len(split.List) != 3 || split.List[0].Text != "" || split.List[1].Text != "boo" || split.List[2].Text != "" {
		t.Fatalf("split = %#v handled=%v err=%v", split, handled, err)
	}
	lookahead, handled, err := callStringMember(String("abc"), "replaceAll", []Value{String("(?=a)"), String("x")})
	if err != nil || !handled || lookahead.Text != "xabc" {
		t.Fatalf("replaceAll lookahead = %#v handled=%v err=%v", lookahead, handled, err)
	}
	var runtimeErr *RuntimeError
	_, _, err = callStringMember(String("abc"), "replaceFirst", []Value{String("(?<word>[a-z]+)"), String("${word}")})
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, "String.replaceFirst Java regex named groups") {
		t.Fatalf("replaceFirst named regex unsupported err = %#v", err)
	}
	_, _, err = callStringMember(String("abc"), "replaceAll", []Value{String("([a-z]+)"), String("${word}")})
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, "String.replaceAll replacement named group references") {
		t.Fatalf("replaceAll named replacement unsupported err = %#v", err)
	}
	_, _, err = callStringMember(String("abc"), "split", []Value{String(`(.)\1`)})
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, "String.split Java regex backreferences") {
		t.Fatalf("split unsupported err = %#v", err)
	}
}

func TestExecPatternCompileSupportsTerminalPositiveLookahead(t *testing.T) {
	program, err := CompileAnonymous(`
Pattern pattern = Pattern.compile('(?i)(?s)\\(\\s*SELECT\\s.*?\\)(?=\\s*,|\\s*FROM\\s|\\s*$)');
Matcher matcher = pattern.matcher('SELECT Id, (SELECT Id FROM Lines__r), Name FROM Order__c');
System.assert(matcher.find());
System.assertEquals('(SELECT Id FROM Lines__r)', matcher.group());
System.assert(!pattern.matcher('(SELECT Id FROM Lines__r) trailing').find());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternMatchesSupportsLeadingNegativeLookaheads(t *testing.T) {
	program, err := CompileAnonymous(`
String passwordPattern = '(?!^[0-9]*$)(?!^[a-zA-Z]*$)^([!-~]{8,50})$';
System.assert(Pattern.matches(passwordPattern, 'abc123!!'));
System.assert(!Pattern.matches(passwordPattern, '12345678'));
System.assert(!Pattern.matches(passwordPattern, 'abcdefgh'));
System.assert(!Pattern.matches(passwordPattern, 'a1!'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStringRegexSplitRejectsNullableEdges(t *testing.T) {
	var runtimeErr *RuntimeError
	for _, pattern := range []string{`\b`, "^"} {
		_, _, err := callStringMember(String("abc"), "split", []Value{String(pattern), Int(-1)})
		if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, "String.split regexes that can match empty strings") {
			t.Fatalf("split %q err = %#v", pattern, err)
		}
	}
	charSplit, handled, err := callStringMember(String("abc"), "split", []Value{String(""), Int(-1)})
	if err != nil || !handled || charSplit.Kind != ValueList || len(charSplit.List) != 4 || charSplit.List[0].Text != "a" || charSplit.List[3].Text != "" {
		t.Fatalf("empty regex split = %#v handled=%v err=%v", charSplit, handled, err)
	}
	split, handled, err := callStringMember(String(""), "split", []Value{String("x")})
	if err != nil || !handled || split.Kind != ValueList || len(split.List) != 1 || split.List[0].Text != "" {
		t.Fatalf("empty no-match split = %#v handled=%v err=%v", split, handled, err)
	}
	parts, err := splitRegex("Pattern.split", "", "Ωb", -1)
	if err != nil || len(parts) != 3 || parts[0] != "Ω" || parts[2] != "" {
		t.Fatalf("Pattern.split empty regex = %#v err=%v", parts, err)
	}
}

func TestStringEscapeJavaLikeOctalAndUnicodeEdges(t *testing.T) {
	source := "\b\t\n\f\r\"'\\/Ω😀"
	if got, want := escapeJavaLike(source, false, false), `\b\t\n\f\r\"'\\/\u03A9\uD83D\uDE00`; got != want {
		t.Fatalf("escapeJavaLike = %q, want %q", got, want)
	}
	if got, want := escapeJavaLike(source, true, true), `\b\t\n\f\r\"\'\\\/\u03A9\uD83D\uDE00`; got != want {
		t.Fatalf("escapeEcmaScript-like = %q, want %q", got, want)
	}
	unescaped, err := unescapeJavaLike("String.unescapeJava", `\141\040A\u03A9\uD83D\uDE00\0`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "a AΩ😀\x00"; unescaped != want {
		t.Fatalf("unescapeJavaLike = %q, want %q", unescaped, want)
	}
	unescaped, err = unescapeJavaLike("String.unescapeEcmaScript", `\/\'\"\\`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `/'"\`; unescaped != want {
		t.Fatalf("unescapeEcmaScript-like = %q, want %q", unescaped, want)
	}
	if got, want := escapeUnicode("A\x00Ω😀"), `A\u0000\u03A9\uD83D\uDE00`; got != want {
		t.Fatalf("escapeUnicode = %q, want %q", got, want)
	}
}

func TestExecBlobEncodingCryptoStdlib(t *testing.T) {
	program, err := CompileAnonymous(`
Blob hello = Blob.valueOf('hello');
System.assertEquals('hello', hello.toString());
System.assertEquals(5, hello.size());
System.assertEquals('68656c6c6f', EncodingUtil.convertToHex(hello));
Blob decodedHex = EncodingUtil.convertFromHex('68656C6C6F');
System.assertEquals('hello', decodedHex.toString());
Blob empty = Blob.valueOf('');
System.assertEquals(0, empty.size());
System.assertEquals('', empty.toString());
System.assertEquals('', EncodingUtil.convertToHex(empty));
Blob emptyHex = EncodingUtil.convertFromHex('');
System.assertEquals(0, emptyHex.size());
Blob binary = EncodingUtil.convertFromHex('00FF7f');
System.assertEquals(3, binary.size());
System.assertEquals('00ff7f', EncodingUtil.convertToHex(binary));
System.assertEquals('aGVsbG8=', EncodingUtil.base64Encode(hello));
Blob decodedBase64 = EncodingUtil.base64Decode('aGVsbG8=');
System.assertEquals('hello', decodedBase64.toString());
System.assertEquals('', EncodingUtil.base64Encode(empty));
Blob emptyBase64 = EncodingUtil.base64Decode('');
System.assertEquals(0, emptyBase64.size());
String urlEncoded = EncodingUtil.urlEncode('A B+Ω', 'UTF-8');
System.assertEquals('A+B%2B%CE%A9', urlEncoded);
System.assertEquals('A B+Ω', EncodingUtil.urlDecode(urlEncoded, 'utf8'));
System.assertEquals('%C3%85+trail', EncodingUtil.urlEncode('Å trail', ' UTF_8 '));
System.assertEquals('Å trail', EncodingUtil.urlDecode('%C3%85+trail', 'Utf-8'));
System.assertEquals('caf%E9+trail', EncodingUtil.urlEncode('café trail', 'ISO-8859-1'));
System.assertEquals('café trail', EncodingUtil.urlDecode('caf%E9+trail', 'latin1'));
System.assertEquals('A%2BB+trail*', EncodingUtil.urlEncode('A+B trail*', 'US_ASCII'));
System.assertEquals('A+B trail*', EncodingUtil.urlDecode('A%2BB+trail*', 'ascii'));
Blob md5 = Crypto.generateDigest('MD5', hello);
Blob sha1 = Crypto.generateDigest('SHA1', hello);
Blob sha256 = Crypto.generateDigest('SHA-256', hello);
Blob sha512 = Crypto.generateDigest('SHA-512', hello);
Blob sha3 = Crypto.generateDigest('SHA3-256', hello);
Blob normalizedSha512 = Crypto.generateDigest(' sha_512 ', hello);
System.assertEquals('5d41402abc4b2a76b9719d911017c592', EncodingUtil.convertToHex(md5));
System.assertEquals('aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d', EncodingUtil.convertToHex(sha1));
System.assertEquals('2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824', EncodingUtil.convertToHex(sha256));
System.assertEquals('9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043', EncodingUtil.convertToHex(sha512));
System.assertEquals('3338be694f50c5f338814986cdf0686453a888b84f424d792af4b9202398f392', EncodingUtil.convertToHex(sha3));
System.assertEquals(EncodingUtil.convertToHex(sha512), EncodingUtil.convertToHex(normalizedSha512));
Blob message = Blob.valueOf('message');
Blob key = Blob.valueOf('key');
Blob hmacMD5 = Crypto.generateMac('hmacMD5', message, key);
Blob hmacSHA1 = Crypto.generateMac('hmacSHA1', message, key);
Blob hmacSHA256 = Crypto.generateMac('HmacSHA256', message, key);
Blob hmacSHA512 = Crypto.generateMac('hmacSHA512', message, key);
Blob normalizedHmacSHA256 = Crypto.generateMac(' HMAC-SHA256 ', message, key);
System.assertEquals('4e4748e62b463521f6775fbf921234b5', EncodingUtil.convertToHex(hmacMD5));
System.assertEquals('2088df74d5f2146b48146caf4965377e9d0be3a4', EncodingUtil.convertToHex(hmacSHA1));
System.assertEquals('6e9ef29b75fffc5b7abae527d58fdadb2fe42e7219011976917343065f58ed4a', EncodingUtil.convertToHex(hmacSHA256));
System.assertEquals(EncodingUtil.convertToHex(hmacSHA256), EncodingUtil.convertToHex(normalizedHmacSHA256));
System.assertEquals('e477384d7ca229dd1426e64b63ebf2d36ebd6d7e669a6735424e72ea6c01d3f8b56eb39c36d8232f5427999b8d1a3f9cd1128fc69f4d75b434216810fa367e98', EncodingUtil.convertToHex(hmacSHA512));
System.assert(Crypto.verifyHmac('hmacSHA256', message, key, hmacSHA256));
System.assert(Crypto.verifyHmac(' HMAC-SHA256 ', message, key, normalizedHmacSHA256));
System.assert(!Crypto.verifyHmac('hmacSHA256', Blob.valueOf('changed'), key, hmacSHA256));
System.assert(Crypto.areEqualConstantTime(hello, Blob.valueOf('hello')));
System.assert(!Crypto.areEqualConstantTime(hello, Blob.valueOf('hullo')));
System.assert(!Crypto.areEqualConstantTime(hello, Blob.valueOf('hello!')));
Blob aes128 = Crypto.generateAESKey(128);
Blob aes192 = Crypto.generateAESKey(192);
Blob aes256 = Crypto.generateAESKey(256);
System.assertEquals(16, aes128.size());
System.assertEquals(24, aes192.size());
System.assertEquals(32, aes256.size());
System.assertEquals('0102030405060708090a0b0c0d0e0f10', EncodingUtil.convertToHex(aes128));
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCryptoRandomDeterministicLocalSequence(t *testing.T) {
	program, err := CompileAnonymous(`
Long first = Crypto.getRandomLong();
Long second = Crypto.getRandomLong();
System.assertEquals(-2152535657050944081, first);
System.assertEquals(7960286522194355700, second);
System.assertNotEquals(first, second);
Integer firstInteger = Crypto.getRandomInteger();
Integer secondInteger = Crypto.getRandomInteger();
System.assertEquals(-2146876081, firstInteger);
System.assertEquals(1917616620, secondInteger);
System.assertNotEquals(firstInteger, secondInteger);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}

	repeated, err := CompileAnonymous(`
System.assertEquals(-2152535657050944081, Crypto.getRandomLong());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(repeated, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCryptoEncryptAESCBCDeterministicLocalSubset(t *testing.T) {
	program, err := CompileAnonymous(`
Blob key = Blob.valueOf('0123456789abcdef0123456789abcdef');
Blob iv = Blob.valueOf('abcdef9876543210');
Blob encrypted = Crypto.encrypt('AES256', key, iv, Blob.valueOf('hello'));
System.assertEquals(16, encrypted.size());
System.assertEquals('93ce19c2c83297061f55dadc424d14c3', EncodingUtil.convertToHex(encrypted));
System.assertEquals('hello', Crypto.decrypt('AES256', key, iv, encrypted).toString());
Blob normalized = Crypto.encrypt(' aes-256 ', key, iv, Blob.valueOf('hello'));
System.assertEquals(EncodingUtil.convertToHex(encrypted), EncodingUtil.convertToHex(normalized));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCryptoManagedIVAndSignatureLocalSubset(t *testing.T) {
	program, err := CompileAnonymous(`
Blob key = Blob.valueOf('0123456789abcdef0123456789abcdef');
Blob encrypted = Crypto.encryptWithManagedIV('AES256', key, Blob.valueOf('hello'));
System.assertEquals(32, encrypted.size());
System.assertEquals('hello', Crypto.decryptWithManagedIV('AES256', key, encrypted).toString());
Blob signature = Crypto.sign('RSA-SHA512', Blob.valueOf('hello'), Blob.valueOf('private'));
System.assert(Crypto.verify('RSA-SHA512', Blob.valueOf('hello'), signature, Blob.valueOf('public')));
System.assert(!Crypto.verify('RSA-SHA512', Blob.valueOf('changed'), signature, Blob.valueOf('public')));
Blob certSignature = Crypto.signWithCertificate('RSA-SHA256', Blob.valueOf('hello'), 'cert');
System.assert(Crypto.verifyWithCertificate('RSA-SHA256', Blob.valueOf('hello'), certSignature, 'cert'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCoreTypeIDURLObjectStdlib(t *testing.T) {
	program, err := CompileAnonymous(`
Type accountType = Type.forName('Account');
Type accountTypeAgain = Type.forName('Account');
Type contactType = Type.forName('Contact');
String accountName = 'Account';
System.assertEquals('Account', accountType.getName());
System.assertEquals('Account', accountType.toString());
System.assert(accountType.equals(accountTypeAgain));
System.assert(!accountType.equals(contactType));
System.assertEquals(accountName.hashCode(), accountType.hashCode());

Id valid = Id.valueOf('001B000001DVM9t');
Id fromId = Id.valueOf(valid);
Id same = Id.valueOf('001B000001DVM9t', false);
Id restored = Id.valueOf('001b000001dvm9tIAH', true);
Id restoredLowerChecksum = Id.valueOf('001b000001dvm9tiah', true);
System.assert(valid.equals(same));
System.assert(valid.equals(fromId));
System.assertEquals('001B000001DVM9t', valid.toString());
System.assertEquals('001B000001DVM9tIAH', String.valueOf(valid));
System.assertEquals('id=001B000001DVM9tIAH', 'id=' + valid);
System.assertEquals('Account', Id.valueOf('001B000001DVM9t').getSObjectType().getDescribe().getName());
System.assertEquals('001B000001DVM9t', valid.to15());
System.assertEquals('001B000001DVM9tIAH', valid.to18());
Id longId = Id.valueOf('001B000001DVM9tIAH');
System.assertEquals('001B000001DVM9t', longId.to15());
System.assertEquals('001B000001DVM9tIAH', longId.to18());
System.assertEquals('001B000001DVM9tIAH', restored.toString());
System.assertEquals('001B000001DVM9tIAH', restoredLowerChecksum.toString());
List<String> ids = new List<String>{valid};
System.assertEquals('001B000001DVM9tIAH', ids[0]);

String text = 'trail';
System.assert(text.equals('trail'));
System.assert(!text.equals('ridge'));
String sameText = 'trail';
System.assertEquals('trail', text.toString());
System.assertEquals(sameText.hashCode(), text.hashCode());
System.assertEquals(text.hashCode(), text.HashCode());
Integer count = 7;
System.assert(count.equals(7));
System.assertEquals('7', count.toString());
List<Integer> left = new List<Integer>();
left.add(1);
List<Integer> right = new List<Integer>();
right.add(1);
System.assert(left.equals(right));
System.assert(left.Equals(right));
System.assertEquals(left.hashCode(), right.hashCode());
System.assertEquals('List[1]', left.toString());

URL base = URL.getOrgDomainUrl();
System.assertEquals('https://local.oaer.example', base.toExternalForm());
System.assertEquals('https', base.getProtocol());
System.assertEquals('local.oaer.example', base.getHost());
System.assertEquals(443, base.getDefaultPort());
System.assertEquals(-1, base.getPort());
URL detailed = new URL('https://example.test:8443/apex/Page?id=001#top');
URL detailedAgain = new URL('https://example.test:8443/apex/Page?id=001#top');
System.assert(detailed.equals(detailedAgain));
System.assertEquals(detailed.hashCode(), detailedAgain.hashCode());
System.assertEquals('example.test', detailed.getHost());
System.assertEquals('example.test:8443', detailed.getAuthority());
System.assertEquals('/apex/Page', detailed.getPath());
System.assertEquals('id=001', detailed.getQuery());
System.assertEquals('top', detailed.getRef());
System.assertEquals('/apex/Page?id=001', detailed.getFile());
System.assertEquals(8443, detailed.getPort());
URL userInfo = new URL('https://user:pass@example.test/path');
System.assertEquals('user:pass@example.test', userInfo.getAuthority());
System.assertEquals('example.test', userInfo.getHost());
URL ftp = new URL('ftp://files.example.test/pub/readme.txt');
System.assertEquals(21, ftp.getDefaultPort());
URL protocolHost = new URL('https', 'example.test', '/trail');
System.assertEquals('https://example.test/trail', protocolHost.toExternalForm());
URL protocolHostPort = new URL('https', 'example.test', 8443, '/ridge');
System.assertEquals('https://example.test:8443/ridge', protocolHostPort.toExternalForm());
URL relative = new URL(detailed, '../Other?x=1');
System.assertEquals('https://example.test:8443/Other?x=1', relative.toExternalForm());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemStaticEqualsAndHashCode(t *testing.T) {
	program, err := CompileAnonymous(`
String text = 'trail';
String sameText = 'trail';
System.assert(System.equals(text, sameText));
System.assert(!System.equals(text, 'ridge'));
System.assert(System.equals(null, null));
System.assert(!System.equals(null, text));
System.assertEquals(text.hashCode(), System.hashCode(text));
List<Integer> left = new List<Integer>{1, 2};
List<Integer> right = new List<Integer>{1, 2};
System.assert(System.equals(left, right));
System.assertEquals(left.hashCode(), System.hashCode(right));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeNamespaceContracts(t *testing.T) {
	program, err := CompileAnonymous(`
Type accountType = Type.forName('Account');
System.assertEquals(null, accountType.getNamespace());
System.assertEquals(null, accountType.getPackageName());
System.assertEquals('System', DmlException.class.getNamespace());
System.assertEquals('System', DmlException.class.getPackageName());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionFoundationContracts(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> numbers = new List<Integer>();
System.assert(numbers.isEmpty());
numbers.add(1);
System.assert(!numbers.isEmpty());
System.assertEquals(1, numbers.remove(0));
numbers.add(2);
numbers.clear();
System.assertEquals(0, numbers.size());
System.assert(numbers.isEmpty());

Set<String> names = new Set<String>();
System.assert(names.isEmpty());
System.assert(names.add('Ada'));
System.assert(!names.add('Ada'));
System.assert(names.containsAll(new List<String>{'Ada'}));
System.assert(names.remove('Ada'));
System.assert(names.isEmpty());
names.add('Grace');
names.clear();
System.assertEquals(0, names.size());

Map<String,Integer> counts = new Map<String,Integer>();
System.assert(counts.isEmpty());
System.assertEquals(null, counts.put('a', 1));
System.assert(counts.containsKey('a'));
System.assert(counts.containsValue(1));
System.assertEquals(1, counts.remove('a'));
System.assertEquals(null, counts.remove('missing'));
System.assert(counts.isEmpty());
counts.put('b', 2);
counts.clear();
System.assertEquals(0, counts.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCoreSystemTimeAndDebugStdlib(t *testing.T) {
	program, err := CompileAnonymous(`
Date today = System.today();
System.assertEquals('2026-05-02', today.format(), 'System.today should use the VM clock');
System.assertEquals(0, Date.newInstance(2026, 5, 1).monthsBetween(Date.newInstance(2026, 5, 31)));
System.assertEquals(1, Date.newInstance(2026, 5, 31).monthsBetween(Date.newInstance(2026, 6, 1)));
System.assertEquals(-12, Date.newInstance(2026, 5, 1).monthsBetween(Date.newInstance(2025, 5, 1)));
System.assertEquals(5, today.Month());
Datetime now = System.now();
System.assertEquals('2026-05-02T12:00:00Z', now.format());
System.assertEquals(1777723200000, System.currentTimeMillis());
System.debug(LoggingLevel.INFO, 'logged with level');
System.debug('logged without level');
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(result.Debug), 2; got != want {
		t.Fatalf("debug lines = %d, want %d: %#v", got, want, result.Debug)
	}
	if result.Debug[0] != "logged with level" || result.Debug[1] != "logged without level" {
		t.Fatalf("debug lines = %#v", result.Debug)
	}
}

func TestParseDatetimeTextAcceptsDateOnlyAtMidnightUTC(t *testing.T) {
	parsed, err := parseDatetimeTextAllowDateOnly("2026-05-02")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := parsed.Format(time.RFC3339), "2026-05-02T00:00:00Z"; got != want {
		t.Fatalf("parsed = %s, want %s", got, want)
	}
}

func TestExecCoreExceptionStdlibMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Exception constructed = new DmlException('blocked');
System.assertEquals('blocked', constructed.getMessage());
System.assertEquals('System.DmlException', constructed.getTypeName());
System.assertEquals('System.DmlException', DmlException.class.getName());
System.assertEquals(0, constructed.getLineNumber());
System.assertEquals('', constructed.getStackTraceString());
System.assertEquals('System.DmlException: blocked', constructed.toString());
Exception noMessage = new DmlException();
System.assertEquals(null, noMessage.getMessage());
Exception systemPrefixed = new System.DmlException('system blocked');
System.assertEquals('System.DmlException', systemPrefixed.getTypeName());
System.assertEquals('System.DmlException: system blocked', systemPrefixed.toString());
Exception allCapsDML = new DMLException('caps blocked');
System.assertEquals('System.DMLException', allCapsDML.getTypeName());
Exception aura = new AuraHandledException('aura blocked');
System.assertEquals('System.AuraHandledException', aura.getTypeName());

String caught = '';
try {
	throw new QueryException('bad query');
} catch (Exception e) {
	caught = e.getTypeName() + ':' + e.getMessage();
	System.assert(e.getLineNumber() > 0, 'caught exceptions should carry a line number');
	String stackTrace = e.getStackTraceString();
	System.assert(stackTrace != '', 'caught exceptions should carry a stack trace');
	System.assertEquals('System.QueryException: bad query', e.toString());
}
System.assertEquals('System.QueryException:bad query', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCoreExceptionCauseStdlibMethods(t *testing.T) {
	program, err := CompileAnonymous(`
Exception outer = new DmlException('outer');
System.assertEquals(null, outer.getCause());
Exception cause = new QueryException('root cause');
Exception returned = outer.initCause(cause);
System.assert(outer.equals(returned));
Exception recovered = outer.getCause();
System.assertEquals('System.QueryException', recovered.getTypeName());
System.assertEquals('root cause', recovered.getMessage());

Boolean repeatCaught = false;
try {
	outer.initCause(null);
} catch (Exception e) {
	repeatCaught = true;
	System.assertEquals('System.IllegalStateException', e.getTypeName());
	System.assertEquals('Can''t overwrite cause', e.getMessage());
}
System.assert(repeatCaught, 'repeat initCause should throw');
System.assertEquals('root cause', outer.getCause().getMessage());

Exception nullable = new DmlException('nullable');
nullable.initCause(null);
System.assertEquals(null, nullable.getCause());
Boolean nullRepeatCaught = false;
try {
	nullable.initCause(cause);
} catch (Exception e) {
	nullRepeatCaught = true;
	System.assertEquals('System.IllegalStateException', e.getTypeName());
}
System.assert(nullRepeatCaught, 'null cause initialization should count');

Exception self = new DmlException('self');
Boolean selfCaught = false;
try {
	self.initCause(self);
} catch (Exception e) {
	selfCaught = true;
	System.assertEquals('System.IllegalArgumentException', e.getTypeName());
	System.assertEquals('Self-causation not permitted', e.getMessage());
}
System.assert(selfCaught, 'self cause should throw');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCoreBuiltinExceptionMatrix(t *testing.T) {
	exceptionNames := []string{
		"AssertException",
		"AsyncException",
		"CalloutException",
		"DmlException",
		"EmailException",
		"ExternalObjectException",
		"IllegalArgumentException",
		"IllegalStateException",
		"InvalidParameterValueException",
		"JSONException",
		"LimitException",
		"ListException",
		"MathException",
		"NoAccessException",
		"NoDataFoundException",
		"NoSuchElementException",
		"NullPointerException",
		"QueryException",
		"RequiredFeatureMissingException",
		"SearchException",
		"SecurityException",
		"SerializationException",
		"SObjectException",
		"StringException",
		"TypeException",
		"VisualforceException",
		"XmlException",
	}
	var source strings.Builder
	source.WriteString("Type exceptionType = Type.forName('Exception');\n")
	source.WriteString("System.assertEquals(null, Type.forName('ImaginaryException'));\n")
	for i, name := range exceptionNames {
		source.WriteString("Type t")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString(" = Type.forName('")
		source.WriteString(name)
		source.WriteString("');\n")
		source.WriteString("System.assert(t")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString(" != null, '")
		source.WriteString(name)
		source.WriteString(" Type.forName should resolve');\n")
		source.WriteString("System.assert(exceptionType.isAssignableFrom(t")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString("), '")
		source.WriteString(name)
		source.WriteString(" should extend Exception');\n")
		source.WriteString("Exception e")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString(" = new ")
		source.WriteString(name)
		source.WriteString("('")
		source.WriteString(name)
		source.WriteString(" message');\n")
		source.WriteString("System.assertEquals('System.")
		source.WriteString(name)
		source.WriteString("', e")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString(".getTypeName());\n")
		source.WriteString("System.assertEquals('System.")
		source.WriteString(name)
		source.WriteString(": ")
		source.WriteString(name)
		source.WriteString(" message', e")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString(".toString());\n")
	}
	program, err := CompileAnonymous(source.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAssertFailureMessageEdges(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "assert null message",
			source: "System.assert(false, null);",
			want:   "assertion failed: null",
		},
		{
			name:   "assertEquals null message",
			source: "System.assertEquals('left', 'right', null);",
			want:   "expected <left>, actual <right>: null",
		},
		{
			name:   "assertNotEquals exception message",
			source: "System.assertNotEquals('same', 'same', new DmlException('duplicate'));",
			want:   "values should not be equal: <same>: System.DmlException: duplicate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("error type = %T, want *RuntimeError", err)
			}
			if runtimeErr.Type != "System.AssertException" || runtimeErr.Message != tt.want {
				t.Fatalf("runtime error = (%q, %q), want (System.AssertException, %q)", runtimeErr.Type, runtimeErr.Message, tt.want)
			}
		})
	}
}

func TestExecSystemDebugArityTypeAndUnsupportedAsyncDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "missing message",
			source: "System.debug();",
			want:   "System.debug expects message or logging level and message",
		},
		{
			name:   "bad first argument",
			source: "System.debug('INFO', 'message');",
			want:   "System.debug expects LoggingLevel as first argument",
		},
		{
			name:   "unsupported abortJob",
			source: "System.abortJob('707000000000001');",
			want:   "unsupported call \"System.abortJob local async scheduling surface\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("error = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecSystemAssertFailureMessagesUseObjectToString(t *testing.T) {
	program, err := CompileAnonymous(`System.assert(false, Message.value());`)
	if err != nil {
		t.Fatal(err)
	}
	messageProgram, err := CompileAnonymous(`return new Message();`)
	if err != nil {
		t.Fatal(err)
	}
	toStringProgram, err := CompileAnonymous(`return 'custom object message';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Message",
		Methods: map[string]Method{
			"value":    {Name: "Message.value", ClassName: "Message", IsStatic: true, ReturnType: "Message", Program: messageProgram},
			"toString": {Name: "Message.toString", ClassName: "Message", ReturnType: "String", Program: toStringProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Type != "System.AssertException" || runtimeErr.Message != "assertion failed: custom object message" {
		t.Fatalf("runtime error = (%q, %q)", runtimeErr.Type, runtimeErr.Message)
	}
}

func TestExecSystemDebugNullAndExceptionFormatting(t *testing.T) {
	program, err := CompileAnonymous(`
System.debug(null);
System.debug(new DmlException('blocked'));
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"null", "System.DmlException: blocked"}
	if len(result.Debug) != len(want) || result.Debug[0] != want[0] || result.Debug[1] != want[1] {
		t.Fatalf("debug lines = %#v, want %#v", result.Debug, want)
	}
}

func TestExecTypeIsAssignableFrom(t *testing.T) {
	program, err := CompileAnonymous(`
Type exceptionType = Type.forName('Exception');
Type dmlType = Type.forName('DmlException');
System.assert(exceptionType.isAssignableFrom(dmlType));
System.assert(!dmlType.isAssignableFrom(exceptionType));
Type systemExceptionType = Type.forName('System', 'Exception');
Type systemDmlType = Type.forName('System', 'DmlException');
System.assert(systemExceptionType.isAssignableFrom(dmlType));
System.assert(exceptionType.isAssignableFrom(systemDmlType));

Type markerType = Type.forName('Marker');
Type childType = Type.forName('Child');
Type parentType = Type.forName('Parent');
System.assert(markerType.isAssignableFrom(childType));
System.assert(parentType.isAssignableFrom(childType));
System.assert(!childType.isAssignableFrom(parentType));
System.assert(childType.isAssignableFrom(childType));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Marker", IsInterface: true}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Parent"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Child", SuperClass: "Parent", Interfaces: []string{"Marker"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestBlobEncodingCryptoStdlibRejectsBadInputs(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{source: "Blob b = Blob.valueOf('abc'); b.size(1);", want: "Blob.size expects 0 arguments"},
		{source: "EncodingUtil.base64Decode('not base64');", want: "EncodingUtil.base64Decode invalid base64 string"},
		{source: "EncodingUtil.convertFromHex('abc');", want: "EncodingUtil.convertFromHex invalid hexadecimal string"},
		{source: "EncodingUtil.convertFromHex('zz');", want: "EncodingUtil.convertFromHex invalid hexadecimal string"},
		{source: "Blob bad = EncodingUtil.convertFromHex('80'); bad.toString();", want: "Blob.toString invalid UTF-8 data"},
		{source: "EncodingUtil.urlEncode('Ω', 'ISO-8859-1');", want: `EncodingUtil.urlEncode charset "ISO-8859-1" cannot encode U+03A9`},
		{source: "EncodingUtil.urlEncode('é', 'US-ASCII');", want: `EncodingUtil.urlEncode charset "US-ASCII" cannot encode U+00E9`},
		{source: "EncodingUtil.urlDecode('%E9', 'ASCII');", want: `EncodingUtil.urlDecode charset "US-ASCII" cannot decode byte 0xE9`},
		{source: "EncodingUtil.urlEncode('x', 'UTF-16');", want: `unsupported call "EncodingUtil.urlEncode charset \"UTF-16\""`},
		{source: "EncodingUtil.urlDecode('x', 'UTF-16');", want: `unsupported call "EncodingUtil.urlDecode charset \"UTF-16\""`},
		{source: "EncodingUtil.urlDecode('%zz', 'UTF-8');", want: "invalid URL escape"},
		{source: "Crypto.areEqualConstantTime(Blob.valueOf('x'), 'x');", want: "Crypto.areEqualConstantTime right expects Blob"},
		{source: "Crypto.generateDigest('SHA-999', Blob.valueOf('x'));", want: `unsupported digest algorithm "SHA-999"`},
		{source: "Crypto.generateMac('hmacSHA999', Blob.valueOf('x'), Blob.valueOf('key'));", want: `unsupported MAC algorithm "hmacSHA999"`},
		{source: "Crypto.generateMac('hmacSHA256', Blob.valueOf('x'), 'key');", want: "Crypto.generateMac privateKey expects Blob"},
		{source: "Crypto.verifyHmac('hmacSHA999', Blob.valueOf('x'), Blob.valueOf('key'), Blob.valueOf('mac'));", want: `unsupported MAC algorithm "hmacSHA999"`},
		{source: "Crypto.verifyHmac('hmacSHA256', Blob.valueOf('x'), Blob.valueOf('key'), 'mac');", want: "Crypto.verifyHmac mac expects Blob"},
		{source: "Crypto.encrypt('AES999', Blob.valueOf('0123456789abcdef'), Blob.valueOf('abcdef9876543210'), Blob.valueOf('x'));", want: `unsupported encryption algorithm "AES999"`},
		{source: "Crypto.encrypt('AES256', Blob.valueOf('short'), Blob.valueOf('abcdef9876543210'), Blob.valueOf('x'));", want: "Crypto.encrypt AES256 privateKey expects 32 bytes, got 5"},
		{source: "Crypto.encrypt(' aes-256 ', Blob.valueOf('short'), Blob.valueOf('abcdef9876543210'), Blob.valueOf('x'));", want: "Crypto.encrypt AES256 privateKey expects 32 bytes, got 5"},
		{source: "Crypto.encrypt('AES128', Blob.valueOf('0123456789abcdef'), Blob.valueOf('short'), Blob.valueOf('x'));", want: "Crypto.encrypt initializationVector expects 16 bytes, got 5"},
		{source: "Crypto.encrypt('AES128', Blob.valueOf('0123456789abcdef'), Blob.valueOf('abcdef9876543210'), 'x');", want: "Crypto.encrypt clearText expects Blob"},
	}
	for _, tc := range tests {
		source := tc.source
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected error for %s", source)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("error for %s = %q, want substring %q", source, err.Error(), tc.want)
		}
	}
}

func TestBlobEncodingCryptoStdlibCryptoSurfaceErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "decryptWithManagedIV",
			src:  "Crypto.decryptWithManagedIV('AES128', Blob.valueOf('0123456789abcdef'), Blob.valueOf('data'));",
			want: "Crypto.decryptWithManagedIV cipherText must include managed IV",
		},
		{
			name: "decrypt",
			src:  "Crypto.decrypt('AES128', Blob.valueOf('short'), Blob.valueOf('abcdef9876543210'), Blob.valueOf('data'));",
			want: "Crypto.decrypt AES128 privateKey expects 16 bytes, got 5",
		},
		{
			name: "encryptWithManagedIV",
			src:  "Crypto.encryptWithManagedIV('AES128', Blob.valueOf('key'), Blob.valueOf('data'));",
			want: "Crypto.encrypt AES128 privateKey expects 16 bytes, got 3",
		},
		{
			name: "sign",
			src:  "Crypto.sign('RSA-SHA999', Blob.valueOf('data'), Blob.valueOf('key'));",
			want: `unsupported signature algorithm "RSA-SHA999"`,
		},
		{
			name: "verify",
			src:  "Crypto.verify('RSA-SHA999', Blob.valueOf('data'), Blob.valueOf('sig'), Blob.valueOf('key'));",
			want: `unsupported signature algorithm "RSA-SHA999"`,
		},
		{
			name: "signWithCertificate",
			src:  "Crypto.signWithCertificate('RSA-SHA999', Blob.valueOf('data'), 'cert');",
			want: `unsupported signature algorithm "RSA-SHA999"`,
		},
		{
			name: "verifyWithCertificate",
			src:  "Crypto.verifyWithCertificate('RSA-SHA999', Blob.valueOf('data'), Blob.valueOf('sig'), 'cert');",
			want: `unsupported signature algorithm "RSA-SHA999"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tc.want {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestExecTypeNewInstanceRunsZeroArgConstructor(t *testing.T) {
	constructorProgram, err := CompileAnonymous(`this.Name = 'built';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Type thingType = Type.forName('Thing');
Thing thing = thingType.newInstance();
System.assertEquals('built', thing.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Thing",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
		FieldOrder: []string{"Name"},
		Constructors: []Method{
			{Name: "Thing.<init>", ClassName: "Thing", Program: constructorProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCoreSystemContextHelpersReturnFalse(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, System.isBatch());
System.assertEquals(false, System.isFuture());
System.assertEquals(false, System.isQueueable());
System.assertEquals(false, System.isScheduled());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecLoggingLevelEnumEdges(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('INFO', LoggingLevel.INFO.name());
System.assertEquals('INFO', LoggingLevel.INFO.toString());
System.assertEquals(3, LoggingLevel.INFO.ordinal());
List<LoggingLevel> levels = LoggingLevel.values();
System.assertEquals(8, levels.size());
LoggingLevel firstLevel = levels.get(0);
LoggingLevel lastLevel = levels.get(7);
System.assertEquals('NONE', firstLevel.name());
System.assertEquals('FINEST', lastLevel.name());
System.debug(LoggingLevel.ERROR, LoggingLevel.WARN);
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Debug; len(got) != 1 || got[0] != "WARN" {
		t.Fatalf("debug lines = %#v, want %#v", got, []string{"WARN"})
	}
}

func TestExecTriggerOperationEnumStaticValues(t *testing.T) {
	program, err := CompileAnonymous(`
TriggerOperation operation = TriggerOperation.AFTER_INSERT;
System.assertEquals('AFTER_INSERT', operation.name());
System.assert(operation == TriggerOperation.AFTER_INSERT);
System.assert(operation != TriggerOperation.BEFORE_INSERT);
Integer matched = 0;
switch on operation {
  when AFTER_INSERT {
    matched = 1;
  }
  when BEFORE_INSERT {
    matched = -1;
  }
}
System.assertEquals(1, matched);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTriggerGlobalsOutsideTriggerUseDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
TriggerOperation operation = Trigger.operationType;
System.assertEquals(null, operation);
System.assertEquals(false, Trigger.isExecuting);
System.assertEquals(false, Trigger.isBefore);
System.assertEquals(null, Trigger.new);
System.assertEquals(0, Trigger.size);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStatusCodeEnumStaticValue(t *testing.T) {
	program, err := CompileAnonymous(`
StatusCode code = StatusCode.FIELD_CUSTOM_VALIDATION_EXCEPTION;
System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', String.valueOf(code));
System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', code.name());
System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION', code.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecIdComparisonUsesLexicalOrder(t *testing.T) {
	program, err := CompileAnonymous(`
Id older = '001000000000001';
Id newer = '001000000000002';
System.assert(newer > older);
System.assert(older < newer);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeForNameNullAndUnknownEdges(t *testing.T) {
	program, err := CompileAnonymous(`
Type nullName = Type.forName(null);
System.assertEquals(null, nullName);
Type blankName = Type.forName('');
System.assertEquals(null, blankName);
Type unknown = Type.forName('DefinitelyMissing');
System.assertEquals(null, unknown);
Type unknownWithNullNamespace = Type.forName(null, 'DefinitelyMissing');
System.assertEquals(null, unknownWithNullNamespace);
Type accountWithNullNamespace = Type.forName(null, 'Account');
System.assertEquals('Account', accountWithNullNamespace.getName());
Type stringType = Type.forName('String');
System.assertEquals('String', stringType.getName());
System.assertEquals('System.String', Type.forName('System.String').getName());
Type systemStringType = Type.forName('System', 'String');
System.assertEquals('System.String', systemStringType.getName());
System.assertEquals(null, Type.forName('System', 'DefinitelyMissing'));
Type systemDmlType = Type.forName('System', 'DmlException');
System.assertEquals('System.DmlException', systemDmlType.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeForNameResolvesGenericCustomSObjectTypes(t *testing.T) {
	program, err := CompileAnonymous(`
Type recordsType = Type.forName('List<Widget__c>');
System.assertEquals('List<Widget__c>', recordsType.getName());
List<SObject> records = (List<SObject>)recordsType.newInstance();
System.assertEquals(0, records.size());
Type mapType = Type.forName('Map<Id, Widget__c>');
System.assertEquals('Map<Id,Widget__c>', mapType.getName());
Type genericMapType = Type.forName('Map<String,sObject>');
System.assertEquals('Map<String,sObject>', genericMapType.getName());
Map<String, SObject> genericMap = (Map<String, SObject>)genericMapType.newInstance();
System.assertEquals(0, genericMap.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__c"}}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomObjectCloneCopiesFields(t *testing.T) {
	program, err := CompileAnonymous(`
EmailMessage original = new EmailMessage();
original.Subject = 'first';
EmailMessage copied = original.clone();
copied.Subject = 'second';
System.assertEquals('first', original.Subject);
System.assertEquals('second', copied.Subject);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "EmailMessage",
		Fields: map[string]Field{
			"Subject": {Name: "Subject", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCoreSystemReflectionEdgesRejectBadInputs(t *testing.T) {
	tests := []string{
		`System.isBatch(true);`,
		`LoggingLevel.values('x');`,
		`LoggingLevel.INFO.name('x');`,
		`Type.forName(1);`,
		`Type.forName(1, 'Account');`,
		`Type.forName(null, 1);`,
	}
	for _, source := range tests {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected error for %s", source)
		}
	}
}

func TestExecDateIsSameDay(t *testing.T) {
	program, err := CompileAnonymous(`
Date day = Date.newInstance(2026, 5, 7);
System.assert(day.isSameDay(Date.newInstance(2026, 5, 7)));
System.assert(day.isSameDay(DateTime.newInstance(2026, 5, 7, 12, 30, 0)));
System.assertEquals(false, day.isSameDay(Date.newInstance(2026, 5, 8)));
System.assertEquals(day, DateTime.newInstance(2026, 5, 7, 0, 0, 0));
System.assertNotEquals(day, DateTime.newInstance(2026, 5, 7, 12, 30, 0));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCoreIDValueOfRejectsInvalidAndRestoreCasing(t *testing.T) {
	tests := []string{
		`Id.valueOf('short');`,
		`Id.valueOf('001B000001DVM9!');`,
		`Id.valueOf('001B000001DVM9tIAA');`,
		`Id.valueOf('001B000001DVM9t999', true);`,
	}
	for _, source := range tests {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected error for %s", source)
		}
	}
}

func TestExecIDGetSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = Id.valueOf('001B000001DVM9t');
Object accountType = accountId.getSObjectType();
Object accountDescribe = accountType.getDescribe();
System.assertEquals('Account', accountDescribe.getName());
Id contactId = Id.valueOf('003B000001DVM9tIAH');
Object contactType = contactId.getSObjectType();
Object contactDescribe = contactType.getDescribe();
System.assertEquals('Contact', contactDescribe.getName());
Id customId = Id.valueOf('a00B000001DVM9t');
Object customType = customId.getSObjectType();
Object customDescribe = customType.getDescribe();
System.assertEquals('Trail__c', customDescribe.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName": {APIName: "LastName", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	org.Objects["Trail__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Trail__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIDGetSObjectTypeUsesStoredRecordWhenPrefixesCollide(t *testing.T) {
	program, err := CompileAnonymous(`
Id scheduleTypeId = Id.valueOf('a2D000000000001');
Object scheduleType = scheduleTypeId.getSObjectType();
Object describe = scheduleType.getDescribe();
System.assertEquals('ScheduleType__mdt', describe.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["BatchDataSource__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "BatchDataSource__mdt", KeyPrefix: "a2D", Fields: map[string]storage.Field{}},
		Records:    make(map[storage.ID]storage.Record),
	}
	org.Objects["ScheduleType__mdt"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "ScheduleType__mdt", KeyPrefix: "a9Z", Fields: map[string]storage.Field{}},
		Records: map[storage.ID]storage.Record{
			"a2D000000000001": {Object: "ScheduleType__mdt", ID: "a2D000000000001"},
		},
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIDMembersAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = Id.valueOf('001B000001DVM9t');
System.assertEquals('001B000001DVM9t', accountId.to15());
System.assertEquals('001B000001DVM9tIAH', accountId.TO18());
System.assertEquals('Account', accountId.getSobjectType().getDescribe().getName());
Object boxedId = accountId;
System.assertEquals('Account', boxedId.getSOBJECTTYPE().getDescribe().getName());
System.assertEquals('Account', Account.SObjectType.GETDESCRIBE().GETNAME());
System.assert(Account.SObjectType.fields.Name.GETDESCRIBE().ISUPDATEABLE());
URL detailed = new URL('https://example.test:8443/apex/Page?id=001#top');
System.assertEquals('https', detailed.GETPROTOCOL());
System.assertEquals('example.test', detailed.getHOST());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectGetSObjectTypeForInstancesAndStaticTypes(t *testing.T) {
	program, err := CompileAnonymous(`
Event_Log__c event = new Event_Log__c();
Object instanceType = event.getSObjectType();
System.assertEquals('Event_Log__c', instanceType.getDescribe().getName());
SObject genericEvent = new Event_Log__c();
System.assertEquals('Event_Log__c', genericEvent.getSObjectType().getDescribe().getName());
System.assertEquals('Event_Log__c', Event_Log__c.getSObjectType().getDescribe().getName());
System.assertEquals('Account', Account.getSObjectType().getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Event_Log__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Event_Log__c",
			KeyPrefix: "a50",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectStaticGetSObjectTypeDoesNotShadowUserStaticMethod(t *testing.T) {
	methodProgram, err := CompileAnonymous(`return 'user method';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String value = Account.getSObjectType();
System.assertEquals('user method', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "BaseAccount"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Account", SuperClass: "BaseAccount"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "BaseAccount.getSObjectType", ClassName: "BaseAccount", ReturnType: "String", IsStatic: true, Program: methodProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIDGetSObjectTypeRejectsUnknownPrefix(t *testing.T) {
	program, err := CompileAnonymous(`
Id unknown = Id.valueOf('999B000001DVM9t');
unknown.getSObjectType();
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err == nil {
		t.Fatal("expected unknown Id prefix error")
	}
	if got, want := err.Error(), "System.StringException: Invalid id prefix: 999"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestExecURLCurrentRequestUrlUsesCurrentPage(t *testing.T) {
	program, err := CompileAnonymous(`
URL defaultURL = URL.getCurrentRequestUrl();
System.assertEquals('https://local.oaer.example/apex/current', defaultURL.toExternalForm());
PageReference current = ApexPages.currentPage();
current.getParameters().put('mode', 'local');
URL withParams = URL.getCurrentRequestUrl();
System.assertEquals('https://local.oaer.example/apex/current?mode=local', withParams.toExternalForm());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeNewInstanceUnsupportedNamespacePackageToken(t *testing.T) {
	program, err := CompileAnonymous(`
Type packaged = Type.forName('pkg', 'Thing');
packaged.newInstance();
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "Type.newInstance namespace/package reflection for pkg.Thing"` {
		t.Fatalf("runtime error = (%q, %q)", runtimeErr.Type, runtimeErr.Message)
	}
}

func TestExecTypeNewInstanceAllowsDottedBuiltins(t *testing.T) {
	program, err := CompileAnonymous(`
Object exceptionValue = Type.forName('System.AssertException').newInstance();
System.assertEquals('System.AssertException', exceptionValue.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeNewInstanceRejectsUninstantiableBuiltins(t *testing.T) {
	program, err := CompileAnonymous(`Type.forName('String').newInstance();`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "Type.newInstance uninstantiable built-in String"` {
		t.Fatalf("runtime error = (%q, %q)", runtimeErr.Type, runtimeErr.Message)
	}
}

func TestExecURLConstructorRejectsMalformedInputs(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{source: `URL u = new URL('trail');`, want: "URL constructor invalid URL: missing protocol"},
		{source: `URL u = new URL('https:///trail');`, want: "URL constructor invalid URL: missing host"},
		{source: `URL u = new URL('https', 'example.test', 70000, '/trail');`, want: "URL constructor invalid URL: invalid port"},
		{source: `URL u = new URL('https', '', '/trail');`, want: "URL constructor invalid URL: missing host"},
	}
	for _, tc := range tests {
		program, err := CompileAnonymous(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected error for %s", tc.source)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("error for %s = %q, want substring %q", tc.source, err.Error(), tc.want)
		}
	}
}

func TestExecNumericStdlibExpansion(t *testing.T) {
	program, err := CompileAnonymous(`
Integer i = Integer.valueOf('42');
Integer signed = Integer.valueOf('  +42 ');
Long l = Long.valueOf('9001');
Long minLong = Long.valueOf(' -9223372036854775808 ');
Long maxLong = Long.valueOf('+9223372036854775807');
Decimal d = Decimal.valueOf('12.5');
Decimal negativeDecimal = Decimal.valueOf(' -0.125 ');
Decimal lowerCaseDecimal = decimal.valueOf('3.5');
Double x = Double.valueOf('2.25');
Double signedDouble = Double.valueOf(' +6.25 ');
Decimal bigLong = Decimal.valueOf('3000000000');
Boolean truthy = Boolean.valueOf(' TRUE ');
Boolean falsy = boolean.valueOf('no');
Boolean nilBool = Boolean.valueOf(null);
Integer nilInt = Integer.valueOf(null);
Long nilLong = Long.valueOf(null);
Decimal nilDecimal = Decimal.valueOf(null);
System.assertEquals(42, i);
System.assertEquals(42, signed);
System.assertEquals(9001, l);
System.assertEquals(Long.MIN_VALUE, minLong);
System.assertEquals(Long.MAX_VALUE, maxLong);
System.assertEquals(12.5, d);
System.assertEquals(-0.125, negativeDecimal);
System.assertEquals(3.5, lowerCaseDecimal);
System.assertEquals(2.25, x);
System.assertEquals(6.25, signedDouble);
System.assertEquals(true, truthy);
System.assertEquals(false, falsy);
System.assertEquals(false, nilBool);
System.assertEquals(null, nilInt);
System.assertEquals(null, nilLong);
System.assertEquals(null, nilDecimal);
System.assertEquals('42', i.format());
System.assertEquals('1,234,567', 1234567.format());
System.assertEquals('9,001', l.format());
System.assertEquals('10,000.5', Decimal.valueOf('10000.5').format());
System.assertEquals('2.25', x.format());
System.assertEquals('', 'abc'.left(-1));
System.assertEquals('', 'abc'.right(-1));
System.assertEquals('10.00', Decimal.valueOf('10').setScale(2).toPlainString());
System.assertEquals(42.0, i.doubleValue());
System.assertEquals(12, d.intValue());
System.assertEquals(12, d.longValue());
System.assertEquals(3000000000, bigLong.longValue());
System.assertEquals(12.5, d.doubleValue());
System.assertEquals(12.5, d.abs());
System.assertEquals(156.25, d.pow(2));
System.assertEquals('12.5', d.format());
System.assertEquals('10000.5', Decimal.valueOf('10000.5').toPlainString());
System.assertEquals(2, Decimal.valueOf('0.010').stripTrailingZeros().scale());
System.assertEquals(1, Decimal.valueOf('0.010').stripTrailingZeros().precision());
System.assertEquals(0, Decimal.valueOf('0.00').stripTrailingZeros().scale());
System.assertEquals(-1, Decimal.valueOf('10').scale() - Decimal.valueOf('10').precision() + 1);
System.assertEquals(10, Decimal.valueOf('12.5').setScale(-1));
Decimal halfTie = Decimal.valueOf('1.25');
Decimal halfEvenUp = Decimal.valueOf('1.35');
Decimal negativeHalfTie = Decimal.valueOf('-1.25');
Decimal positiveRoundUp = Decimal.valueOf('1.21');
Decimal positiveRoundDown = Decimal.valueOf('1.29');
Decimal negativeDirected = Decimal.valueOf('-1.21');
Decimal roundEvenDown = Decimal.valueOf('12.5');
Decimal roundEvenUp = Decimal.valueOf('13.5');
Decimal roundUnneeded = Decimal.valueOf('12.0');
System.assertEquals(1.3, halfTie.setScale(1, RoundingMode.valueOf('HALF_UP')));
System.assertEquals(1.2, halfTie.setScale(1, RoundingMode.valueOf('HALF_DOWN')));
System.assertEquals(1.2, halfTie.setScale(1, RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(1.4, halfEvenUp.setScale(1, RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(-1.2, negativeHalfTie.setScale(1, RoundingMode.valueOf('HALF_DOWN')));
System.assertEquals(1.3, positiveRoundUp.setScale(1, RoundingMode.valueOf('UP')));
System.assertEquals(1.2, positiveRoundDown.setScale(1, RoundingMode.valueOf('DOWN')));
System.assertEquals(-1.2, negativeDirected.setScale(1, RoundingMode.valueOf('CEILING')));
System.assertEquals(-1.3, negativeDirected.setScale(1, RoundingMode.valueOf('FLOOR')));
System.assertEquals(12, roundEvenDown.round(RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(14, roundEvenUp.round(RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(12, roundUnneeded.round(RoundingMode.valueOf('UNNECESSARY')));
System.assertEquals('HALF_UP', RoundingMode.HALF_UP.name());
System.assertEquals('HALF_UP', RoundingMode.valueOf('HALF_UP').toString());
System.assertEquals(0, RoundingMode.UP.ordinal());
System.assertEquals(7, RoundingMode.UNNECESSARY.ordinal());
List<RoundingMode> roundingModes = RoundingMode.values();
System.assertEquals(8, roundingModes.size());
System.assertEquals('UP', roundingModes.get(0).name());
System.assertEquals('UNNECESSARY', roundingModes.get(7).name());
System.assertEquals(0.33, Decimal.valueOf('1.00').divide(3, 2, System.RoundingMode.HALF_UP));
System.assertEquals(1, Math.signum(12.5));
System.assertEquals(-1, Math.signum(-4));
System.assertEquals(0, Math.signum(0));
System.assertEquals(1, Math.mod(10, 3));
System.assertEquals(2.5, Math.mod(12.5, 5));
System.assertEquals(12, Math.roundToLong(12.5));
System.assertEquals(3.0, Math.ceil(2.1));
System.assertEquals(2.0, Math.floor(2.9));
System.assertEquals(2, Math.round(2.5));
System.assertEquals(2.0, Math.rint(2.5));
System.assertEquals(7, Math.max(3, 7));
System.assertEquals(3, Math.min(3, 7));
System.assertEquals(3.0, Math.sqrt(9));
System.assertEquals(2.0, Math.cbrt(8));
System.assertEquals(8.0, Math.pow(2, 3));
System.assertEquals(2147483647, Integer.MAX_VALUE);
System.assertEquals(-2147483648, Integer.MIN_VALUE);
System.assert(Long.MAX_VALUE > 0);
System.assert(Long.MIN_VALUE < 0);
System.assert(Math.abs(Math.PI - 3.141592653589793) < 0.000000000000001);
System.assert(Math.abs(Math.E - 2.718281828459045) < 0.000000000000001);
System.assert(Math.abs(Math.sin(Math.PI / 2) - 1) < 0.000000000001);
System.assert(Math.abs(Math.cos(0) - 1) < 0.000000000001);
System.assert(Math.abs(Math.tan(0)) < 0.000000000001);
System.assert(Math.abs(Math.cosh(0) - 1) < 0.000000000001);
System.assert(Math.abs(Math.sinh(0)) < 0.000000000001);
System.assert(Math.abs(Math.tanh(0)) < 0.000000000001);
System.assert(Math.abs(Math.acos(1)) < 0.000000000001);
System.assert(Math.abs(Math.asin(1) - (Math.PI / 2)) < 0.000000000001);
System.assert(Math.abs(Math.atan(1) - (Math.PI / 4)) < 0.000000000001);
System.assert(Math.abs(Math.atan2(1, 1) - (Math.PI / 4)) < 0.000000000001);
System.assert(Math.abs(Math.exp(1) - Math.E) < 0.000000000001);
System.assert(Math.abs(Math.log(Math.E) - 1) < 0.000000000001);
System.assert(Math.abs(Math.log10(1000) - 3) < 0.000000000001);
System.assert(Math.random() >= 0);
System.assert(Math.random() < 1);
String uuid1 = UUID.randomUUID();
String uuid2 = UUID.randomUUID();
System.assertNotEquals(uuid1, uuid2);
System.assertEquals(36, uuid1.length());
System.assertEquals('4', uuid1.substring(14, 15));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecBuiltinStaticCallsAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
system.assertEquals(8.0, math.Pow(2, 3));
SYSTEM.ASSERTEQUALS(3.0, MATH.sqrt(9));
System.assertEquals(4, database.CountQuery('SELECT count() FROM Account'));
System.assertEquals(Date.today(), date.TODAY());
System.assertEquals('INFO', logginglevel.info.name());
System.assertEquals('HALF_UP', roundingmode.half_up.name());
System.assertEquals(1.3, Decimal.valueOf('1.25').setScale(1, roundingmode.half_up));
System.assertEquals(false, test.ISRUNNINGTEST());
System.assertEquals(false, System.Test.isRunningTest());
System.System.debug('default namespace System class call');
System.assertEquals(Date.today(), System.Date.today());
System.assertEquals(1, System.Limits.GETQUERIES());
System.assertEquals(16, crypto.generateAesKey(128).size());
Map<String,Object> parsed = (Map<String,Object>)System.JSON.deserializeUntyped('{"ok":true}');
System.assertEquals(true, (Boolean)parsed.get('ok'));
System.assertEquals('HALF_UP', System.RoundingMode.HALF_UP.name());
System.assertEquals(UserInfo.getUserId(), userinfo.GETUSERID());
System.assertEquals(UserInfo.getUserId(), System.UserInfo.getUserId());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account"},
			"001000000000002": {ID: "001000000000002", Object: "Account"},
			"001000000000003": {ID: "001000000000003", Object: "Account"},
			"001000000000004": {ID: "001000000000004", Object: "Account"},
		},
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemNamespaceBuiltinBeatsShadowingGlobal(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, System.Test.isRunningTest());
System.assertEquals(true, Test.Database.hasRecords());
`)
	if err != nil {
		t.Fatal(err)
	}
	hasRecordsProgram, err := CompileAnonymous("return true;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "MockDatabase",
		Methods: map[string]Method{
			"hasRecords": {Name: "MockDatabase.hasRecords", ClassName: "MockDatabase", ReturnType: "Boolean", Program: hasRecordsProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	testFactory := Object("TestFactory")
	testFactory.Fields["Database"] = Object("MockDatabase")
	machine.Globals["Test"] = testFactory

	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecProjectStaticResourceSOQL(t *testing.T) {
	root := filepath.Join("..", "..", "example-projects", "src-nmb-nutpl-develop")
	if _, err := os.Stat(filepath.Join(root, "sfdx-project.json")); err != nil {
		t.Skip("example project is not available")
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err := resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String recordName = 'resetcss';
Set<String> names = new Set<String>{ recordName };
Set<String> escaped = new Set<String>();
for (String name : names) {
	escaped.add(String.escapeSingleQuotes(name));
}
System.assertEquals(1, escaped.size());
String query = String.format('SELECT {0} FROM {1} WHERE Name in :escaped ORDER BY {2}', new List<String>{ 'Body,Name,NamespacePrefix,SystemModStamp', 'StaticResource', 'Name' });
List<StaticResource> resources = Database.query(query);
System.assertEquals(1, resources.size());
System.assertEquals('resetcss', resources[0].Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericStdlibRejectsInvalidInputs(t *testing.T) {
	tests := []string{
		"Integer.valueOf('not an integer');",
		"Integer.valueOf('  ');",
		"Integer.valueOf('42.0');",
		"Integer.valueOf('2147483648');",
		"Long.valueOf('9x');",
		"Long.valueOf('9223372036854775808');",
		"Decimal.valueOf('NaN');",
		"Decimal.valueOf('-Infinity');",
		"Decimal.valueOf('1e309');",
		"Double.valueOf('NaN');",
		"Double.valueOf('Infinity');",
		"Double.valueOf('1,234.5');",
		"Decimal d = Decimal.valueOf('3000000000');\nd.intValue();",
		"Decimal d = Decimal.valueOf('1.25');\nd.setScale(1, LoggingLevel.ERROR);",
		"Decimal d = Decimal.valueOf('1.25');\nd.round(RoundingMode.valueOf('UNNECESSARY'));",
		"RoundingMode.valueOf('HALF_CEILING');",
		"RoundingMode.valueOf(' HALF_UP ');",
		"RoundingMode.values('HALF_UP');",
		"RoundingMode.UP.name('extra');",
		"Decimal d = Decimal.valueOf('1e308') * Decimal.valueOf('1e308');",
		"Math.acos(2);",
		"Math.asin(-2);",
		"Math.sqrt(-1);",
		"Math.log(0);",
		"Math.log10(-1);",
		"Math.exp(1000);",
		"Math.pow(10, 1000);",
	}
	for _, source := range tests {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected numeric stdlib error for %s", source)
		}
	}
}

func TestExecNumericValueOfParseErrorsAreTypeExceptions(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caughtInteger = false;
try {
	Integer.valueOf('not an integer');
} catch (System.TypeException e) {
	caughtInteger = true;
}
Boolean caughtLong = false;
try {
	Long.valueOf('not a long');
} catch (TypeException e) {
	caughtLong = true;
}
System.assert(caughtInteger);
System.assert(caughtLong);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalScaleFenceUnsupported(t *testing.T) {
	program, err := CompileAnonymous("Decimal d = Decimal.valueOf('1.25');\nd.setScale(16);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected Decimal.setScale scale fence error")
	} else if !strings.Contains(err.Error(), `unsupported call "Decimal.setScale absolute scale greater than 15 is not supported by the local decimal model"`) {
		t.Fatalf("Decimal.setScale scale fence error = %q", err.Error())
	}
}

func TestExecDecimalScaleUsesLocalDecimalStringTies(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal nickel = Decimal.valueOf('1.005');
Decimal bankersDown = Decimal.valueOf('2.685');
Decimal bankersUp = Decimal.valueOf('2.675');
Decimal negative = Decimal.valueOf('-1.005');
System.assertEquals(1.01, nickel.setScale(2));
System.assertEquals(1.00, nickel.setScale(2, RoundingMode.valueOf('HALF_DOWN')));
System.assertEquals(2.68, bankersDown.setScale(2, RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(2.68, bankersUp.setScale(2, RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(-1.01, negative.setScale(2, RoundingMode.valueOf('HALF_UP')));
System.assertEquals(3, Decimal.valueOf('2.5').round());
System.assertEquals(-3, Decimal.valueOf('-2.5').round());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericStdlibRejectsIntegerOverflow(t *testing.T) {
	tests := []string{
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nInteger.valueOf(d);",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nd.intValue();",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nd.longValue();",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nd.round();",
		"Decimal d = Decimal.valueOf('99999999999999999999999.5');\nMath.roundToLong(d);",
		"Long.MAX_VALUE + 1;",
		"Long.MIN_VALUE - 1;",
		"Long.MAX_VALUE * 2;",
		"-Long.MIN_VALUE;",
		"Long.MIN_VALUE / -1;",
		"Math.abs(Long.MIN_VALUE);",
	}
	for _, source := range tests {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Execute(program, nil); err == nil {
			t.Fatalf("expected overflow error for %s", source)
		}
	}
}

func TestExecNumericFormatOverloadsAreUnsupported(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{source: "Integer i = 7;\ni.format('en_US');", want: `unsupported call "Integer/Long.format locale/pattern overloads"`},
		{source: "Long l = 7;\nl.format('en_US');", want: `unsupported call "Integer/Long.format locale/pattern overloads"`},
		{source: "Decimal d = 7.25;\nd.format('en_US');", want: `unsupported call "Decimal/Double.format locale/pattern overloads"`},
		{source: "Double d = 7.25;\nd.format('en_US');", want: `unsupported call "Decimal/Double.format locale/pattern overloads"`},
	}
	for _, tc := range tests {
		program, err := CompileAnonymous(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Execute(program, nil)
		if err == nil {
			t.Fatalf("expected unsupported numeric format overload error for %s", tc.source)
		}
		var runtimeErr *RuntimeError
		if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
			t.Fatalf("expected UnsupportedFeature for %s, got %T %v", tc.source, err, err)
		}
		if runtimeErr.Message != tc.want {
			t.Fatalf("error message = %q, want %q", runtimeErr.Message, tc.want)
		}
	}
}

func TestExecCollectionStdlibExpansion(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2};
System.assert(!xs.isEmpty());
xs.add(3);
xs.add(1, 9);
System.assertEquals(4, xs.size());
System.assertEquals(9, xs.get(1));
System.assertEquals(1, xs.indexOf(9));
System.assertEquals(-1, xs.indexOf(99));
System.assertEquals(9, xs.remove(1));
xs.set(0, 7);
List<Integer> more = new List<Integer>{8, 9};
xs.addAll(more);
System.assertEquals(7, xs.get(0));
System.assertEquals(9, xs.get(xs.size() - 1));
xs.clear();
System.assert(xs.isEmpty());
System.assertEquals(0, xs.size());

Set<String> names = new Set<String>{'a'};
System.assert(!names.isEmpty());
System.assert(names.add('b'));
System.assert(!names.add('b'));
System.assert(names.containsAll(new List<String>{'a', 'b'}));
System.assert(names.remove('a'));
System.assert(!names.contains('a'));
System.assert(names.addAll(new List<String>{'c', 'd'}));
System.assert(names.removeAll(new Set<String>{'b'}));
System.assert(names.retainAll(new List<String>{'c'}));
System.assert(names.contains('c'));
System.assertEquals(1, names.size());
names.clear();
System.assert(names.isEmpty());
System.assertEquals(0, names.size());

Map<String,Integer> counts = new Map<String,Integer>();
System.assert(counts.isEmpty());
System.assertEquals(null, counts.put('a', 1));
System.assertEquals(1, counts.put('a', 2));
System.assert(!counts.isEmpty());
System.assert(counts.containsValue(2));
Set<String> keys = counts.keySet();
System.assert(keys.contains('a'));
List<Integer> values = counts.values();
System.assert(values.contains(2));
Map<String,Integer> moreCounts = new Map<String,Integer>();
moreCounts.put('b', 3);
counts.putAll(moreCounts);
System.assertEquals(3, counts.get('b'));
System.assertEquals(2, counts.remove('a'));
System.assertEquals(null, counts.remove('missing'));
counts.clear();
System.assert(counts.isEmpty());
System.assert(!counts.containsKey('b'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibAcceptsLowercaseGenericTypeNames(t *testing.T) {
	program, err := CompileAnonymous(`
list<String> matchedBindings = new list<String>();
System.assert(matchedBindings.isEmpty());
matchedBindings.add('binding');
System.assert(!matchedBindings.isEmpty());

set<String> names = new set<String>();
System.assert(names.isEmpty());
names.add('acme');
System.assert(!names.isEmpty());

map<String, Integer> counts = new map<String, Integer>();
System.assert(counts.isEmpty());
counts.put('acme', 1);
System.assert(!counts.isEmpty());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibMoreMethods(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> source = new List<Integer>{3, 1, 2};
List<Integer> copied = new List<Integer>(source);
source.set(0, 9);
System.assertEquals(3, copied.size());
System.assertEquals(3, copied.get(0));
copied.sort();
System.assertEquals(1, copied.get(0));
System.assertEquals(3, copied.get(2));
List<Integer> cloned = copied.clone();
cloned.set(0, 7);
System.assertEquals(1, copied.get(0));
System.assertEquals(7, cloned.get(0));
List<Integer> deep = copied.deepClone();
deep.set(1, 8);
System.assertEquals(2, copied.get(1));
System.assertEquals(8, deep.get(1));
List<Integer> deepWithOptions = copied.deepClone(true, true, true);
deepWithOptions.set(2, 9);
System.assertEquals(3, copied.get(2));
System.assertEquals(9, deepWithOptions.get(2));

List<String> words = new List<String>{'delta', 'alpha', 'charlie'};
words.sort();
System.assertEquals('alpha', words.get(0));
System.assertEquals('delta', words.get(2));

Set<String> fromList = new Set<String>(new List<String>{'b', 'a', 'b'});
System.assertEquals(2, fromList.size());
Set<String> setClone = fromList.clone();
setClone.add('c');
System.assertEquals(2, fromList.size());
System.assertEquals(3, setClone.size());
Set<String> setDeep = fromList.deepClone();
setDeep.remove('a');
System.assert(fromList.contains('a'));
System.assert(!setDeep.contains('a'));

Map<String,Integer> counts = new Map<String,Integer>();
counts.put('b', 2);
counts.put('a', 1);
Map<String,Integer> copiedCounts = new Map<String,Integer>(counts);
System.assertEquals(counts, copiedCounts);
System.assertEquals('Map{a=1, b=2}', copiedCounts.toString());
List<Integer> orderedValues = copiedCounts.values();
System.assertEquals(1, orderedValues.get(0));
System.assertEquals(2, orderedValues.get(1));
Map<String,Integer> clonedCounts = copiedCounts.clone();
clonedCounts.put('a', 9);
System.assertEquals(1, copiedCounts.get('a'));
System.assertEquals(9, clonedCounts.get('a'));
Map<String,Integer> deepCounts = copiedCounts.deepClone();
deepCounts.put('b', 8);
System.assertEquals(2, copiedCounts.get('b'));
System.assertEquals(8, deepCounts.get('b'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCollectionStdlibMoreRejectsUnsupportedSortValues(t *testing.T) {
	values := []Value{Map()}
	err := New(nil).sortComparableValues(values, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
		t.Fatalf("err = %T %v, want UnsupportedFeature", err, err)
	}
	if !strings.Contains(runtimeErr.Message, "List.sort for non-primitive comparable values") {
		t.Fatalf("error message = %q", runtimeErr.Message)
	}
}

func TestExecListSortUsesApexComparable(t *testing.T) {
	compareProgram, err := CompileAnonymous(`
Box otherBox = (Box) other;
if (this.Rank == otherBox.Rank) {
	return 0;
}
if (this.Rank > otherBox.Rank) {
	return 1;
}
return -1;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box high = new Box();
high.Rank = 2;
Box low = new Box();
low.Rank = 1;
List<Box> boxes = new List<Box>{high, low};
boxes.sort();
System.assertEquals(1, boxes.get(0).Rank);
System.assertEquals(2, boxes.get(1).Rank);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Box",
		Interfaces: []string{"Comparable"},
		Fields: map[string]Field{
			"Rank": {Name: "Rank", Type: "Integer"},
		},
		Methods: map[string]Method{
			"compareTo": {
				Name:       "Box.compareTo",
				ClassName:  "Box",
				ReturnType: "Integer",
				Params:     []Param{{Name: "other", Type: "Object"}},
				Program:    compareProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSortUsesApexComparator(t *testing.T) {
	compareProgram, err := CompileAnonymous(`
if (left == right) {
	return 0;
}
if (left < right) {
	return 1;
}
return -1;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Integer> nums = new List<Integer>{2, 1, 3};
nums.sort(new DescComparator());
System.assertEquals(3, nums.get(0));
System.assertEquals(2, nums.get(1));
System.assertEquals(1, nums.get(2));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "DescComparator",
		Interfaces: []string{"Comparator<Integer>"},
		Methods: map[string]Method{
			"compare": {
				Name:       "DescComparator.compare",
				ClassName:  "DescComparator",
				ReturnType: "Integer",
				Params:     []Param{{Name: "left", Type: "Integer"}, {Name: "right", Type: "Integer"}},
				Program:    compareProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSortUsesSelectOptionLabelValue(t *testing.T) {
	program, err := CompileAnonymous(`
List<SelectOption> options = new List<SelectOption>{
	new SelectOption('b', 'Beta'),
	new SelectOption('a2', 'Alpha'),
	new SelectOption('a1', 'Alpha')
};
options.sort();
System.assertEquals('a1', options[0].getValue());
System.assertEquals('a2', options[1].getValue());
System.assertEquals('b', options[2].getValue());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSortUsesPlatformScalarOrdering(t *testing.T) {
	program, err := CompileAnonymous(`
List<Date> dates = new List<Date>{
	Date.newInstance(2026, 5, 14),
	Date.newInstance(2025, 1, 1),
	Date.newInstance(2026, 1, 1)
};
dates.sort();
System.assertEquals(Date.newInstance(2025, 1, 1), dates[0]);
System.assertEquals(Date.newInstance(2026, 1, 1), dates[1]);
System.assertEquals(Date.newInstance(2026, 5, 14), dates[2]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSortSupportsSObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Account beta = new Account(Id = '001000000000002AAA', Name = 'Beta');
Account acme = new Account(Id = '001000000000001AAA', Name = 'Acme');
List<Account> accounts = new List<Account>{beta, acme};
accounts.sort();
System.assertEquals('001000000000001AAA', accounts.get(0).Id);
System.assertEquals('001000000000002AAA', accounts.get(1).Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibIterators(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2, 3};
Iterator<Integer> it = xs.iterator();
System.assert(it.hasNext());
System.assertEquals(1, it.next());
System.assertEquals(2, it.next());
xs.add(4);
System.assertEquals(3, it.next());
System.assert(!it.hasNext());

Set<String> names = new Set<String>{'b', 'a'};
Iterator<String> nameIt = names.iterator();
System.assert(nameIt.hasNext());
System.assertEquals('b', nameIt.next());
System.assertEquals('a', nameIt.next());
System.assert(!nameIt.hasNext());

Iterator<String> splitIt = 'CreatedBy.Name'.split('\\.').iterator();
System.assertEquals('CreatedBy', splitIt.next());
System.assertEquals('Name', splitIt.next());

System.Iterator<Integer> systemIt = xs.iterator();
System.assert(systemIt.hasNext());
System.assertEquals(1, systemIt.next());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibIteratorErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "next exhausted",
			body: "List<Integer> xs = new List<Integer>(); Iterator<Integer> it = xs.iterator(); it.next();",
			want: "NoSuchElementException: Iterator has no more elements",
		},
		{
			name: "remove unsupported",
			body: "List<Integer> xs = new List<Integer>{1}; Iterator<Integer> it = xs.iterator(); it.remove();",
			want: "unsupported call \"Iterator.remove\"",
		},
		{
			name: "object sort unsupported",
			body: "List<PageReference> pages = new List<PageReference>{new PageReference('/a')}; pages.sort();",
			want: "unsupported call \"List.sort for non-primitive comparable values\"",
		},
		{
			name: "set deepClone options unsupported",
			body: "Set<Account> accounts = new Set<Account>{new Account(Id = '001B000001DVM9tIAH')}; accounts.deepClone(true);",
			want: "unsupported call \"Set.deepClone with preserve options\"",
		},
		{
			name: "map deepClone options unsupported",
			body: "Map<Id, Account> accounts = new Map<Id, Account>(); accounts.deepClone(true, true, true);",
			want: "unsupported call \"Map.deepClone with preserve options\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestExecCollectionStdlibDeepCloneNestedSObjects(t *testing.T) {
	program, err := CompileAnonymous(`
Account acme = new Account(Id = '001B000001DVM9tIAH', Name = 'Acme');
Account beta = new Account(Id = '001B000001DVM9uIAH', Name = 'Beta');
List<Account> accounts = new List<Account>{acme, beta};
List<Account> clonedAccounts = accounts.deepClone();
Account clonedAcme = clonedAccounts.get(0);
clonedAcme.Name = 'Clone';
Account originalAcme = accounts.get(0);
Account clonedAcmeAgain = clonedAccounts.get(0);
System.assertEquals('Acme', originalAcme.Name);
System.assertEquals('Clone', clonedAcmeAgain.Name);

Map<Id, Account> byId = new Map<Id, Account>(accounts);
Map<Id, Account> clonedById = byId.deepClone();
Account fromClone = clonedById.get(acme.Id);
fromClone.Name = 'Mapped Clone';
Account originalMapped = byId.get(acme.Id);
Account clonedMapped = clonedById.get(acme.Id);
System.assertEquals('Acme', originalMapped.Name);
System.assertEquals('Mapped Clone', clonedMapped.Name);

Set<Account> originalSet = new Set<Account>{acme};
Set<Account> clonedSet = originalSet.deepClone();
Iterator<Account> clonedIt = clonedSet.iterator();
Account clonedFromSet = clonedIt.next();
clonedFromSet.Name = 'Set Clone';
Iterator<Account> originalIt = originalSet.iterator();
Account originalFromSet = originalIt.next();
System.assertEquals('Acme', originalFromSet.Name);
System.assertEquals('Set Clone', clonedFromSet.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibSmallRowsCloseout(t *testing.T) {
	program, err := CompileAnonymous(`
List<Boolean> flags = new List<Boolean>{true, false, true};
flags.sort();
System.assertEquals(false, flags.get(0));
System.assertEquals(true, flags.get(2));

Map<String,Object> shape = new Map<String,Object>();
shape.put('b', new List<Integer>{2, 3});
shape.put('a', null);
System.assertEquals('Map{a=null, b=List[2, 3]}', shape.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCollectionStdlibCloneValueNestedCollections(t *testing.T) {
	account := Object("Account")
	account.Fields["Name"] = String("Acme")
	nested := List(Map())
	nested.List[0].Map["accounts"] = List(account)

	cloned := cloneValue(nested)
	cloned.List[0].Map["accounts"].List[0].Fields["Name"] = String("Clone")

	if got := nested.List[0].Map["accounts"].List[0].Fields["Name"].Text; got != "Acme" {
		t.Fatalf("original nested SObject name = %q, want Acme", got)
	}
	if got := cloned.List[0].Map["accounts"].List[0].Fields["Name"].Text; got != "Clone" {
		t.Fatalf("cloned nested SObject name = %q, want Clone", got)
	}
}

func TestCollectionStdlibCloneValueBreaksCycles(t *testing.T) {
	parent := Object("Node")
	child := Object("Node")
	parent.Fields["Child"] = child
	child.Fields["Parent"] = parent

	cloned := cloneValue(parent)
	if cloned.Fields["Child"].Fields["Parent"].Fields != nil {
		t.Fatalf("cycle clone should stop at repeated reference")
	}
	if cloned.Fields["Child"].Fields["Parent"].Type != "Node" {
		t.Fatalf("cycle placeholder type = %q, want Node", cloned.Fields["Child"].Fields["Parent"].Type)
	}
}

func TestExecCollectionStdlibNullAndSObjectMapEdges(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> words = new List<String>{null, 'a', null};
System.assert(words.contains(null));
System.assertEquals(0, words.indexOf(null));
System.assertEquals(-1, words.indexOf('missing'));
System.assertEquals(null, words.remove(0));
System.assertEquals(2, words.size());

Set<String> names = new Set<String>{null, 'a', null};
System.assert(names.contains(null));
System.assert(!names.add(null));
System.assert(names.remove(null));
System.assert(!names.contains(null));
System.assert(!names.isEmpty());
names.clear();
System.assert(names.isEmpty());

Map<String, String> labels = new Map<String, String>();
labels.put(null, 'nil');
labels.put('blank', null);
System.assert(labels.containsKey(null));
System.assert(labels.containsValue(null));
System.assertEquals('nil', labels.get(null));
Set<String> nullKeys = labels.keySet();
System.assert(nullKeys.contains(null));
System.assertEquals(null, labels.remove('blank'));
System.assert(!labels.containsValue(null));
labels.clear();
System.assert(labels.isEmpty());

Account a = new Account(Id = '001B000001DVM9tIAH', Name = 'Acme');
Account b = new Account(Id = '001B000001DVM9uIAH', Name = 'Beta');
List<Account> accounts = new List<Account>{a, b};
Map<Id, Account> byId = new Map<Id, Account>(accounts);
System.assertEquals(2, byId.size());
Account fromById = byId.get(a.Id);
System.assertEquals('Acme', fromById.Name);
System.assert(byId.containsKey(b.Id));
Map<Id, Account> more = new Map<Id, Account>();
more.putAll(new List<Account>{b});
Account fromMore = more.get(b.Id);
System.assertEquals('Beta', fromMore.Name);

List<Account> maybeAccounts = null;
Map<Id, Account> emptyById = new Map<Id, Account>(maybeAccounts);
System.assert(emptyById.isEmpty());
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibRejectsSObjectMapEdgeErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing Id",
			body: "List<Account> accounts = new List<Account>{new Account(Name = 'Acme')}; Map<Id, Account> byId = new Map<Id, Account>(accounts);",
			want: "requires non-null Id at index 0",
		},
		{
			name: "duplicate Id",
			body: "Account a = new Account(Id = '001B000001DVM9tIAH'); List<Account> accounts = new List<Account>{a, a}; Map<Id, Account> byId = new Map<Id, Account>(accounts);",
			want: "duplicate Id at index 1",
		},
		{
			name: "null SObject row",
			body: "List<Account> accounts = new List<Account>{null}; Map<Id, Account> byId = new Map<Id, Account>(accounts);",
			want: "requires non-null SObject at index 0",
		},
		{
			name: "wrong SObject value type",
			body: "List<Account> accounts = new List<Account>{new Account(Id = '001B000001DVM9tIAH')}; Map<Id, Contact> byId = new Map<Id, Contact>(accounts);",
			want: "value at index 0: cannot assign Account to Contact",
		},
		{
			name: "putAll duplicate Id",
			body: "Account a = new Account(Id = '001B000001DVM9tIAH'); List<Account> accounts = new List<Account>{a, a}; Map<Id, Account> byId = new Map<Id, Account>(); byId.putAll(accounts);",
			want: "duplicate Id at index 1",
		},
		{
			name: "wrong map key type",
			body: "List<Account> accounts = new List<Account>{new Account(Id = '001B000001DVM9tIAH')}; Map<String, Account> byId = new Map<String, Account>(accounts);",
			want: "unsupported call \"Map constructor from SObject list\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}
