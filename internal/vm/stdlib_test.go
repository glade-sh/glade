package vm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
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
System.assertEquals('abcdef', s.toLowerCase('en_US'));
System.assertEquals('ABCDEF', s.toUpperCase('en_US'));
String lowerName = 'hello maximillian';
String upperName = 'Hello max';
System.assertEquals('Hello maximillian', lowerName.capitalize());
System.assertEquals('hello max', upperName.uncapitalize());
System.assertEquals('cDef', s.substring(2));
System.assertEquals('cD', s.substring(2, 4));
System.assertEquals('bcD', s.Substring(1, 4));
try {
  s.substring(0, -1);
  System.assert(false, 'expected substring to throw');
} catch (StringException e) {
  System.assert(e.getMessage().contains('out of bounds'));
}
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
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringSubstringUsesApexUTF16Indexes(t *testing.T) {
	program, err := CompileAnonymous(`
String text = 'a😀b';
System.assertEquals(4, text.length());
System.assertEquals('😀', text.substring(1, 2));
System.assertEquals('', text.substring(2, 3));
System.assertEquals('😀', text.substring(1, 3));
System.assertEquals('b', text.substring(3));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
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
System.assertEquals('hello', root.getText());
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
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDomXmlNodeTypeEnumAndDomNamespaceMembers(t *testing.T) {
	program, err := CompileAnonymous(`
Dom.XmlNodeType elementType = Dom.XmlNodeType.ELEMENT;
System.assertEquals(Dom.XmlNodeType.ELEMENT, elementType);
System.assertEquals(0, elementType.ordinal());
System.assertEquals('ELEMENT', elementType.name());
System.assertEquals('ELEMENT', elementType.toString());
System.assertEquals(true, elementType.equals(Dom.XmlNodeType.ELEMENT));
System.assertEquals(false, elementType.equals(Dom.XmlNodeType.TEXT));
System.assertNotEquals(0, elementType.hashCode());

Dom.XmlNodeType textType = Dom.XmlNodeType.TEXT;
System.assertEquals(1, textType.ordinal());
System.assertEquals('TEXT', textType.name());

Dom.XmlNodeType commentType = Dom.XmlNodeType.COMMENT;
System.assertEquals(2, commentType.ordinal());
System.assertEquals('COMMENT', commentType.name());

List<Dom.XmlNodeType> allTypes = Dom.XmlNodeType.values();
System.assertEquals(3, allTypes.size());
System.assertEquals('ELEMENT', allTypes[0].name());
System.assertEquals('TEXT', allTypes[1].name());
System.assertEquals('COMMENT', allTypes[2].name());

Dom.XmlNodeType fromName = Dom.XmlNodeType.valueOf('TEXT');
System.assertEquals(Dom.XmlNodeType.TEXT, fromName);
System.assertEquals(1, fromName.ordinal());

Dom.XmlNodeType fromLower = Dom.XmlNodeType.valueOf('text');
System.assertEquals(Dom.XmlNodeType.TEXT, fromLower);

Dom.XmlNodeType lowerText = dom.XmlNodeType.TEXT;
System.assertEquals(Dom.XmlNodeType.TEXT, lowerText);
System.assertEquals(1, lowerText.ordinal());

Dom.XmlNodeType lowerComment = dom.XmlNodeType.COMMENT;
System.assertEquals(Dom.XmlNodeType.COMMENT, lowerComment);
System.assertEquals(2, lowerComment.ordinal());

Dom.Document doc1 = new Dom.Document();
doc1.load('<root><child>data</child></root>');
String docStr = doc1.toString();
System.assert(docStr.length() > 0);

Dom.Document doc2 = new Dom.Document();
doc2.load('<root><child>data</child></root>');
System.assert(doc1.hashCode() != 0);
System.assert(doc2.hashCode() != 0);

Dom.XmlNode built = new Dom.XmlNode();
System.assertEquals(Dom.XmlNodeType.ELEMENT, built.getNodeType());

Dom.Document builtDoc = new Dom.Document();
Dom.XmlNode root = builtDoc.createRootElement('root', null, null);
Dom.XmlNode child1 = root.addChildElement('a', null, null);
Dom.XmlNode child2 = root.addChildElement('b', null, null);
System.assertEquals(2, root.getChildElements().size());

System.assertEquals('a', child1.getName());

Dom.XmlNode newKid = root.addChildElement('x', null, null);
System.assertEquals('x', newKid.getName());
System.assertEquals(3, root.getChildElements().size());

Dom.XmlNode inserted = root.insertBefore(new Dom.XmlNode(), child2);
System.assertEquals(Dom.XmlNodeType.ELEMENT, inserted.getNodeType());
System.assertEquals(4, root.getChildElements().size());

System.assertEquals(true, root.removeChild(inserted));
System.assertEquals(3, root.getChildElements().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubQueryRowsBuildsSObjectsFromMaps(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Object> one = new Map<String, Object>{'Id' => '001000000000001', 'Name' => 'Acme'};
Account row = (Account)Test.createStubQueryRow(Account.SObjectType, one);
System.assertEquals('001000000000001', row.Id);
System.assertEquals('Acme', row.Name);

List<Map<String, Object>> rows = new List<Map<String, Object>>{
    one,
    new Map<String, Object>{'Id' => '001000000000002', 'Name' => 'Global Media'}
};
List<SObject> stubbed = Test.createStubQueryRows(Account.SObjectType, rows);
System.assertEquals(2, stubbed.size());
System.assertEquals('Global Media', ((Account)stubbed[1]).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateSoqlStubDispatchesProvider(t *testing.T) {
	providerProgram, err := CompileAnonymous(`
Map<String, Object> row = new Map<String, Object>{'Id' => '001000000000123', 'Name' => query + ':' + String.valueOf(binds.get('name'))};
return Test.createStubQueryRows(targetType, new List<Map<String, Object>>{row});
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.createSoqlStub(Account.SObjectType, new Provider());
System.assert(Test.isSoqlStubDefined(Account.SObjectType));
List<Account> inlineRows = [SELECT Id, Name FROM Account];
System.assertEquals(1, inlineRows.size());
System.assertEquals('001000000000123', inlineRows[0].Id);
List<Account> rows = Database.queryWithBinds('SELECT Id, Name FROM Account WHERE Name = :name', new Map<String,Object>{'name' => 'Acme'});
System.assertEquals(1, rows.size());
System.assertEquals('001000000000123', rows[0].Id);
System.assert(rows[0].Name.contains('SELECT Id, Name FROM Account WHERE Name ='));
System.assert(rows[0].Name.endsWith(':Acme'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"SoqlStubProvider"},
		Methods: map[string]Method{
			"handleSoqlQuery": {
				Name:       "Provider.handleSoqlQuery",
				ClassName:  "Provider",
				ReturnType: "List<SObject>",
				Params: []Param{
					{Name: "targetType", Type: "Schema.SObjectType"},
					{Name: "query", Type: "String"},
					{Name: "binds", Type: "Map<String,Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateSoqlStubDispatchesRelationshipQueriesToProvider(t *testing.T) {
	providerProgram, err := CompileAnonymous(`
Map<String, Object> row = new Map<String, Object>{'Id' => '001000000000123', 'Name' => query};
return Test.createStubQueryRows(targetType, new List<Map<String, Object>>{row});
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.createSoqlStub(Account.SObjectType, new Provider());
List<Account> childRows = [SELECT Id, Name, (SELECT Id FROM Contacts) FROM Account];
System.assertEquals(1, childRows.size());
System.assert(childRows[0].Name.contains('Contacts'));

List<Account> semiJoinRows = [SELECT Id, Name FROM Account WHERE Id IN (SELECT AccountId FROM Contact)];
System.assertEquals(1, semiJoinRows.size());
System.assert(semiJoinRows[0].Name.contains('SELECT AccountId FROM Contact'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"SoqlStubProvider"},
		Methods: map[string]Method{
			"handleSoqlQuery": {
				Name:       "Provider.handleSoqlQuery",
				ClassName:  "Provider",
				ReturnType: "List<SObject>",
				Params: []Param{
					{Name: "targetType", Type: "Schema.SObjectType"},
					{Name: "query", Type: "String"},
					{Name: "binds", Type: "Map<String,Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateSoqlStubRejectsUnsupportedQueryShapes(t *testing.T) {
	providerProgram, err := CompileAnonymous(`return new List<SObject>();`)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "aggregate",
			src:  `Integer total = [SELECT COUNT() FROM Account];`,
			want: `unsupported call "Test.createSoqlStub aggregate query local stub surface"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{
				Name:       "Provider",
				Interfaces: []string{"SoqlStubProvider"},
				Methods: map[string]Method{
					"handleSoqlQuery": {
						Name:       "Provider.handleSoqlQuery",
						ClassName:  "Provider",
						ReturnType: "List<SObject>",
						Params: []Param{
							{Name: "targetType", Type: "Schema.SObjectType"},
							{Name: "query", Type: "String"},
							{Name: "binds", Type: "Map<String,Object>"},
						},
						Program: providerProgram,
					},
				},
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.testCreateSoqlStub([]Value{sObjectTypeToken("Account"), Object("Provider")}); err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != tc.want {
				t.Fatalf("err = %#v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecTestLoadDataInsertsStaticResourceCSV(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> rows = Test.loadData(Account.SObjectType, 'Accounts');
System.assertEquals(2, rows.size());
System.assert(rows[0].Id != null);
System.assertEquals('Acme', rows[0].Name);
System.assertEquals(2, [SELECT Id FROM Account].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Metadata.StaticResources = []storage.StaticResourceMetadata{{
		Name:    "Accounts",
		Content: "Name\nAcme\nGlobal Media\n",
	}}
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
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

func TestExecRestRequestAndResponseConstructorDefaultsMatchSalesforce(t *testing.T) {
	program, err := CompileAnonymous(`
RestRequest req = new RestRequest();
System.assertEquals(null, req.requestURI);
System.assertEquals(null, req.resourcePath);
System.assertEquals(null, req.httpMethod);
System.assertEquals(null, req.remoteAddress);
System.assertEquals(null, req.requestBody);
System.assertEquals(0, req.headers.size());
System.assertEquals(0, req.params.size());

RestResponse res = new RestResponse();
System.assertEquals(null, res.statusCode);
System.assertEquals(0, res.headers.size());
System.assertEquals(null, res.responseBody);
`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatalf("execute: %v", err)
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

func TestExecPageReferenceGetContentReturnsUnsupportedFeature(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/apex/Trail');
page.getContent();
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "PageReference.getContent local Visualforce page rendering surface"` {
		t.Fatalf("err = %#v, want UnsupportedFeature PageReference.getContent", err)
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
page.setCookies(new List<Cookie>{new Cookie('theme', 'dark', null, 100, false)});
System.Cookie systemCookie = new System.Cookie('theme', 'dark', null, 100, false);
System.assertEquals('theme', systemCookie.getName());
System.assertEquals(null, systemCookie.getPath());
System.assertEquals('001000000000001', page.getParameters().get('id'));
System.assertEquals('001000000000001', page.getparameters().get('id'));
System.assertEquals('yes', page.getHeaders().get('X-Local'));
System.assertEquals(2, page.getCookies().size());
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

func TestExecPageReferenceParameterClearUpdatesURL(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = new PageReference('/apex/Trail?id=001000000000001&mode=edit');
page.getParameters().clear();
System.assertEquals('/apex/Trail', page.getUrl());
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
System.assertEquals(37.7749, System.Location.newInstance(37.7749, -122.4194).getLatitude());
System.assert(address.equals(address));
System.assert(!address.equals(right));
QueueableDuplicateSignature sig = QueueableDuplicateSignature.builder()
	.addString('job')
	.addInteger(42)
	.addId('001000000000001AAA')
	.build();
System.assert(sig.toString().contains('String:job'));
System.assert(sig.toString().contains('Integer:42'));
System.assert(sig.toString().contains('Id:001000000000001AAA'));
Builder aliasBuilder = new Builder();
System.assertEquals(0, aliasBuilder.getSize());
System.assert(aliasBuilder.getMaxSize() > 0);
System.assert(aliasBuilder.getRemainingSize() > 0);
QueueableDuplicateSignature aliasSig = aliasBuilder.addString('alias').build();
System.assert(aliasSig.toString().contains('String:alias'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSmallPlatformHelperValueObjects(t *testing.T) {
	program, err := CompileAnonymous(`
CURRENCY usd = CURRENCY.newInstance(12.5, 'usd');
System.assertEquals('USD 12.5', usd.format());
System.assertEquals('12.5', usd.formatAmount());
System.assertEquals('USD 12.5', usd.toString());
Collator collator = Collator.getInstance();
System.assert(collator.compare('a', 'b') < 0);
System.assertEquals(0, collator.compare('same', 'same'));
String threadId = Cases.generateThreadingMessageId('500000000000001AAA');
System.assert(threadId.contains('500000000000001AAA'));
System.assertEquals('500000000000001AAA', String.valueOf(Cases.getCaseIdFromEmailThreadId(threadId)));
System.assertEquals(null, Cases.getCaseIdFromEmailThreadId('no case id here'));
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
System.assertEquals('glade.my.salesforce.local', orgHost);
System.assertEquals('glade.setup.local', setupHost);
System.assertEquals('pkg--glade.visualforce.local', vfHost);
Domain orgDomain = DomainParser.parse(orgHost);
Domain vfDomain = DomainParser.parse('https://' + vfHost + '/apex/Home');
Domain urlDomain = DomainParser.parse(new URL('https://Example.TEST/apex/Home'));
System.assertEquals('glade', orgDomain.getMyDomainName());
System.assertEquals('', orgDomain.getPackageName());
System.assertEquals(null, orgDomain.getSandboxName());
System.assertEquals('pkg', vfDomain.getPackageName());
System.assertEquals('example.test', urlDomain.toString());
System.assertEquals('pkg--glade.visualforce.local', vfDomain.toString());
System.assertEquals('glade.my.salesforce.local', new Domain().toString());
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

pref_center.LoadFormData formData = new pref_center.LoadFormData(new Map<String,pref_center.FieldProperties>());
formData.setTextValue('email', 'person@example.test');
formData.setTextHint('email', 'Email');
formData.setButtonLabel('submit', 'Save');
formData.addOption('frequency', 'weekly', 'Weekly');
formData.addSelectedOption('frequency', 'weekly');
Map<String,Object> formValues = formData.getAsMap();
Map<String,Object> textValues = (Map<String,Object>)formValues.get('textValues');
Map<String,Object> textHints = (Map<String,Object>)formValues.get('textHints');
Map<String,Object> buttonLabels = (Map<String,Object>)formValues.get('buttonLabels');
Map<String,Object> optionsByField = (Map<String,Object>)formValues.get('options');
Map<String,Object> selectedByField = (Map<String,Object>)formValues.get('selectedOptions');
List<SelectOption> options = (List<SelectOption>)optionsByField.get('frequency');
List<String> selected = (List<String>)selectedByField.get('frequency');
System.assertEquals('person@example.test', (String)textValues.get('email'));
System.assertEquals('Email', (String)textHints.get('email'));
System.assertEquals('Save', (String)buttonLabels.get('submit'));
System.assertEquals('weekly', options.get(0).getValue());
System.assertEquals('Weekly', options.get(0).getLabel());
System.assertEquals('weekly', selected.get(0));

CartExtension.Cart utilCart = CartExtension.CartTestUtil.createCart(CartExtension.WebStoreTypeEnum.B2B);
CartExtension.Cart fetchedCart = CartExtension.CartTestUtil.getCart('0aB000000000001AAA');
System.assertNotEquals(null, utilCart);
System.assertNotEquals(null, fetchedCart);
System.assertEquals(CartExtension.WebStoreTypeEnum.B2B, utilCart.webStoreType);
System.assertEquals('0aB000000000001AAA', fetchedCart.id);
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

ConnectApi.NamedCredentialInput namedCredential = new ConnectApi.NamedCredentialInput();
namedCredential.developerName = 'googleBooksAPIApex';
namedCredential.type = ConnectApi.NamedCredentialType.SecuredEndpoint;
namedCredential.calloutOptions = new ConnectApi.NamedCredentialCalloutOptionsInput();
namedCredential.calloutOptions.generateAuthorizationHeader = true;
System.assertEquals('googleBooksAPIApex', (String)namedCredential.developerName);
System.assertEquals(ConnectApi.NamedCredentialType.SecuredEndpoint, namedCredential.type);
System.assertEquals('SecuredEndpoint', ConnectApi.NamedCredentialType.SecuredEndpoint.name());
System.assertEquals(2, ConnectApi.NamedCredentialType.SecuredEndpoint.ordinal());
System.assertEquals(3, ConnectApi.NamedCredentialType.values().size());
System.assertEquals(ConnectApi.NamedCredentialType.PrivateEndpoint, ConnectApi.NamedCredentialType.values()[1]);
System.assertEquals(ConnectApi.CredentialAuthenticationProtocol.Custom, ConnectApi.CredentialAuthenticationProtocol.valueOf('Custom'));
System.assertEquals(5, ConnectApi.CredentialAuthenticationProtocol.values().size());
System.assertEquals(true, namedCredential.calloutOptions.generateAuthorizationHeader);

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

func TestExecPushUpgradeCustomizationRepositoryUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
PushUpgradeCustomizationRepository.create('pkg', '00Dsubscriber', true);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), `unsupported call "PushUpgradeCustomizationRepository.create local push-upgrade customization repository service"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecGeneratedConnectApiServiceCallsNowSupported(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.FeedElement element = ConnectApi.ChatterFeeds.postFeedElement(null, null);
System.assertNotEquals(null, element);
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecConnectApiReadOnlyHarnessReturnsTypedDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.FeedElementPage feed = ConnectApi.ChatterFeeds.getFeedElementsFromFeed(null, null);
System.assertNotEquals(null, feed);

ConnectApi.ChatterLike commentLike =
	ConnectApi.ChatterFeeds.likeComment('community-a', 'comment-a');
System.assertNotEquals(null, commentLike);

ConnectApi.ChatterLike feedElementLike =
	ConnectApi.ChatterFeeds.likeFeedElement('community-a', 'feed-element-a');
System.assertNotEquals(null, feedElementLike);

ConnectApi.ChatterLike feedItemLike =
	ConnectApi.ChatterFeeds.likeFeedItem('community-a', 'feed-item-a');
System.assertNotEquals(null, feedItemLike);

ConnectApi.PollCapability feedElementPoll =
	ConnectApi.ChatterFeeds.voteOnFeedElementPoll('community-a', 'feed-element-a', 'choice-a');
System.assertNotEquals(null, feedElementPoll);

ConnectApi.FeedPoll feedPoll =
	ConnectApi.ChatterFeeds.voteOnFeedPoll('community-a', 'feed-item-a', 'choice-a');
System.assertNotEquals(null, feedPoll);

ConnectApi.FeedElement sharedElement =
	ConnectApi.ChatterFeeds.shareFeedElement('community-a', UserInfo.getUserId(), null, 'feed-element-a');
System.assertNotEquals(null, sharedElement);

ConnectApi.FeedItem sharedItem =
	ConnectApi.ChatterFeeds.shareFeedItem('community-a', null, UserInfo.getUserId(), 'feed-item-a');
System.assertNotEquals(null, sharedItem);

ConnectApi.Subscription groupFollow =
	ConnectApi.ChatterGroups.follow('community-a', 'group-a', UserInfo.getUserId());
System.assertNotEquals(null, groupFollow);

ConnectApi.GroupMembershipRequest groupMembership =
	ConnectApi.ChatterGroups.requestGroupMembership('community-a', 'group-a');
System.assertNotEquals(null, groupMembership);

ConnectApi.ChatterConversationSummary conversation =
	ConnectApi.ChatterMessages.markConversationRead('conversation-a', true);
System.assertNotEquals(null, conversation);

ConnectApi.Subscription userFollow =
	ConnectApi.ChatterUsers.follow('community-a', UserInfo.getUserId(), 'subject-a');
System.assertNotEquals(null, userFollow);

ConnectApi.ManagedContentVersionCollection content =
	ConnectApi.ManagedContent.getAllManagedContent(null, 0, 10, 'en_US', 'News');
System.assertNotEquals(null, content);

ConnectApi.ManagedContentDeliveryChannelsRepresentation channels =
	ConnectApi.ManagedContentDelivery.getChannels(0, 10);
System.assertNotEquals(null, channels);

ConnectApi.AnnouncementPage announcements =
	ConnectApi.Announcements.getAnnouncements('community-a', 'zone-a');
System.assertNotEquals(null, announcements);

ConnectApi.FeedFavorites favorites =
	ConnectApi.ChatterFavorites.getFavorites('community-a', 'me');
System.assertNotEquals(null, favorites);

ConnectApi.ManagedContentChannelsRepresentation managedChannels =
	ConnectApi.ManagedContentChannels.getManagedContentChannels(0, 10, true);
System.assertNotEquals(null, managedChannels);

ConnectApi.NamedCredentialList namedCredentials =
	ConnectApi.NamedCredentials.getNamedCredentials();
System.assertNotEquals(null, namedCredentials);

ConnectApi.GiftWrapProductCollection giftWrapProducts =
	ConnectApi.CommerceCatalog.getGiftWrapProducts('store-a');
System.assertNotEquals(null, giftWrapProducts);

ConnectApi.SortRulesCollection sortRules =
	ConnectApi.CommerceSearch.getSortRules('store-a');
System.assertNotEquals(null, sortRules);

ConnectApi.CdpQueryMetadataOutput cdpMetadata =
	ConnectApi.CdpQuery.getAllMetadata();
System.assertNotEquals(null, cdpMetadata);

ConnectApi.QuerySqlOutput sqlOutput =
	ConnectApi.CdpQuery.querySql(new ConnectApi.QuerySqlInput());
System.assertNotEquals(null, sqlOutput);

ConnectApi.QuerySqlStatus sqlStatus =
	ConnectApi.CdpQuery.querySqlStatus('query-a');
System.assertNotEquals(null, sqlStatus);

ConnectApi.CdpCalculatedInsightPage calculatedInsights =
	ConnectApi.CdpCalculatedInsight.getCalculatedInsights('space-a', 0, 10, null, null);
System.assertNotEquals(null, calculatedInsights);

ConnectApi.CdpCalculatedInsightStandardActionResponseRepresentation calculatedInsightValidation =
	ConnectApi.CdpCalculatedInsight.validateCalculatedInsight(new ConnectApi.CdpCalculatedInsightValidateInput());
System.assertNotEquals(null, calculatedInsightValidation);

ConnectApi.CdpOptimizationDefinitionCollectionRepresentation optimizations =
	ConnectApi.CdpOptimizationConnectApi.getOptimizationDefinitions();
System.assertNotEquals(null, optimizations);

ConnectApi.CdpOptimizationDataModelObjectQueryCountRepresentation optimizationCount =
	ConnectApi.CdpOptimizationConnectApi.postDataModelObjectQueryCount(
		'space-a',
		'object-a',
		new ConnectApi.CdpOptimizationSourceDataInputRepresentation());
System.assertNotEquals(null, optimizationCount);

ConnectApi.CdpSegmentContainerOutput segments =
	ConnectApi.CdpSegment.getSegments();
System.assertNotEquals(null, segments);

ConnectApi.CdpQuickAttributesCollectionRepresentation quickAttributes =
	ConnectApi.CdpQuickAttributes.getQuickAttributes('space-a', 0, null, 10, null);
System.assertNotEquals(null, quickAttributes);

ConnectApi.EinsteinPromptRecordCollectionOutputRepresentation promptTemplates =
	ConnectApi.EinsteinLLM.getPromptTemplates(null, null, 0, 10, new List<String>(), null, null, false);
System.assertNotEquals(null, promptTemplates);

ConnectApi.AudienceCollection audiences =
	ConnectApi.Personalization.getAudiences('site-a', null, null, null, null, false, new List<String>());
System.assertNotEquals(null, audiences);

ConnectApi.SmartDataDiscoveryAIModelCollection aiModels =
	ConnectApi.SmartDataDiscovery.getAIModels();
System.assertNotEquals(null, aiModels);

ConnectApi.CommunityPage communities =
	ConnectApi.Communities.getCommunities();
System.assertNotEquals(null, communities);

ConnectApi.ModerationFlags commentFlags =
	ConnectApi.CommunityModeration.getFlagsOnComment('community-a', 'comment-a');
System.assertNotEquals(null, commentFlags);

ConnectApi.RecordAlertCollectionRepresentation recordAlerts =
	ConnectApi.RecordAlert.getRecordAlerts('001000000000001', '500000000000001');
System.assertNotEquals(null, recordAlerts);

ConnectApi.Motif motif =
	ConnectApi.Records.getMotif('community-a', '001');
System.assertNotEquals(null, motif);

ConnectApi.PicklistValuesCollection picklistValues =
	ConnectApi.RecordUi.getPicklistValuesByRecordType('Account', '012000000000000AAA');
System.assertNotEquals(null, picklistValues);

ConnectApi.RecordAccessDetailRepresentation accessDetail =
	ConnectApi.Sharing.getRecordAccessDetail('001000000000001', UserInfo.getUserId(), 10);
System.assertNotEquals(null, accessDetail);

ConnectApi.UserProfile userProfile =
	ConnectApi.UserProfiles.getUserProfile('community-a', UserInfo.getUserId());
System.assertNotEquals(null, userProfile);

ConnectApi.FeedEntityIsEditable editable =
	ConnectApi.ChatterFeeds.isCommentEditableByMe('community-a', 'comment-a');
System.assertNotEquals(null, editable);

ConnectApi.ActionPlanTemplateItemsOutput actionPlanItems =
	ConnectApi.ActionPlan.getActionPlanTemplateItems('template-a');
System.assertNotEquals(null, actionPlanItems);

ConnectApi.ActionLinkDiagnosticInfo actionLinkInfo =
	ConnectApi.ActionLinks.getActionLinkDiagnosticInfo('community-a', 'action-a');
System.assertNotEquals(null, actionLinkInfo);

ConnectApi.BuyerProfileDetail buyerProfile =
	ConnectApi.CommerceBuyerExperience.getBuyerProfile('webstore-a');
System.assertNotEquals(null, buyerProfile);

ConnectApi.CartSummary cartSummary =
	ConnectApi.CommerceCart.getCartSummary('webstore-a', 'account-a', 'cart-a');
System.assertNotEquals(null, cartSummary);

ConnectApi.WishlistsSummary wishlistSummaries =
	ConnectApi.CommerceWishlist.getWishlistSummaries('webstore-a', 'account-a', true);
System.assertNotEquals(null, wishlistSummaries);

ConnectApi.OrderSummaryAdjustmentAggregatesAsyncOutput adjustmentAggregates =
	ConnectApi.CommerceBuyerExperience.calculateAdjustmentAggregates('webstore-a', null);
System.assertNotEquals(null, adjustmentAggregates);

ConnectApi.CalculateCartResult calculateCartResult =
	ConnectApi.CommerceCart.calculateCart('webstore-a', 'cart-a', 'account-a');
System.assertNotEquals(null, calculateCartResult);

ConnectApi.InventoryCheckAvailabilityOutputRepresentation inventoryAvailability =
	ConnectApi.CommerceInventory.checkInventoryAvailability('scope-a', null);
System.assertNotEquals(null, inventoryAvailability);

ConnectApi.PromotionEvaluation promotionEvaluation =
	ConnectApi.CommercePromotions.evaluate(null);
System.assertNotEquals(null, promotionEvaluation);

ConnectApi.ProductPrice productPrice =
	ConnectApi.CommerceStorePricing.getProductPrice('webstore-a', 'product-a', 'account-a');
System.assertNotEquals(null, productPrice);

ConnectApi.OCIGetInventoryAvailabilityOutputRepresentation ociAvailability =
	ConnectApi.OmnichannelInventoryService.getInventoryAvailability(null);
System.assertNotEquals(null, ociAvailability);

ConnectApi.OCIUploadInventoryAvailabilityStatusOutputRepresentation ociUploadStatus =
	ConnectApi.OmnichannelInventoryService.getInventoryAvailabilityUploadStatus('upload-a');
System.assertNotEquals(null, ociUploadStatus);

ConnectApi.PreviewCancelOutputRepresentation previewCancel =
	ConnectApi.OrderSummary.previewCancel('order-summary-a', null);
System.assertNotEquals(null, previewCancel);

ConnectApi.ProductDetailsOutputRepresentation productDetails =
	ConnectApi.Repricing.productDetails('webstore-a', 'sku-a', 'account-a', 'USD', 'en_US');
System.assertNotEquals(null, productDetails);

ConnectApi.ProductSearchOutputRepresentation repricingSearch =
	ConnectApi.Repricing.searchProducts('webstore-a', 'sku', 0, 10, 'account-a', null);
System.assertNotEquals(null, repricingSearch);

ConnectApi.MngEventCollectionRepresentation managedEvents =
	ConnectApi.EventManagementApis.getMngEvents(null, null, false, null, null, null, null, false, null, 10, false, null, false, null, 0, null, false);
System.assertNotEquals(null, managedEvents);

ConnectApi.ExampleEntityRepresentation exampleEntity =
	ConnectApi.Example.getExampleEntityWithFields('example-a', new List<String>());
System.assertNotEquals(null, exampleEntity);

ConnectApi.ExampleDinosaurOutputRepresentation dinosaur =
	ConnectApi.ExampleIDLApiFamily.getAbstract();
System.assertNotEquals(null, dinosaur);

ConnectApi.ExternalManagedAccountCollectionOutput externalAccounts =
	ConnectApi.ExternalManagedAccount.getExternalManagedAccounts('community-a');
System.assertNotEquals(null, externalAccounts);

ConnectApi.OrchestrationInstance orchestration =
	ConnectApi.Orchestration.getOrchestrationInstance('orchestration-a');
System.assertNotEquals(null, orchestration);

ConnectApi.ValidationMessageRepresentation guardrailValidation =
	ConnectApi.Guardrail.postValidateGuardrail();
System.assertNotEquals(null, guardrailValidation);

ConnectApi.Form marketingForm =
	ConnectApi.MarketingIntegration.getForm('site-a', 'form-a');
System.assertNotEquals(null, marketingForm);

ConnectApi.BotVersionActivationInfo botActivation =
	ConnectApi.BotVersionActivation.getVersionActivationInfo('version-a');
System.assertNotEquals(null, botActivation);

ConnectApi.EventTypesOutput eventTypes =
	ConnectApi.EvfSdk.getEventTypes();
System.assertNotEquals(null, eventTypes);

ConnectApi.EmailMergeFieldInfo mergeFields =
	ConnectApi.EmailMergeFieldService.getMergeFields(new List<String>());
System.assertNotEquals(null, mergeFields);

ConnectApi.FlowApprovalProcessCollection approvals =
	ConnectApi.FlowApprovalProcesses.getFlowApprovalProcessWithStatus('flow-a', new List<String>());
System.assertNotEquals(null, approvals);

ConnectApi.SampleManagementOutputRepresentation sampleSpec =
	ConnectApi.ManufacturingSampleManagement.getProductRequirementSpecification('spec-a');
System.assertNotEquals(null, sampleSpec);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedConnectApiSetTestFixtureReturnsLocalResult(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.FeedElementPage expected = new ConnectApi.FeedElementPage();
ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed('community-a', null, expected);
ConnectApi.FeedElementPage actual = ConnectApi.ChatterFeeds.getFeedElementsFromFeed('community-a', null);
System.assertEquals(expected, actual);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDataWeaveScriptResultCarriers(t *testing.T) {
	program, err := CompileAnonymous(`
DataWeave.Script script = DataWeave.Script.createScript('helloWorld');
DataWeave.Result result = script.execute(new Map<String,Object>());
System.assertEquals('"Hello World"', result.getValueAsString());
System.assertEquals('text/plain', result.getMimeType());
System.assertEquals('"Hello World"', (String)result.valueAsString);
System.assertEquals('"Hello World"', script.execute().getValueAsString());

Map<String,Object> inputs = new Map<String,Object>{'records' => new List<String>{'a', 'b'}};
DataWeave.Result projected = DataWeave.Script.createScript('records').execute(inputs);
System.assertEquals(2, ((List<String>)projected.getValue()).size());

dataweave.Script namespaced = dataweave.Script.createScript('localNs', 'helloWorld');
System.assertEquals('"Hello World"', namespaced.execute().getValueAsString());
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
List<Object> objectFormatArgs = new List<Object>{ Account.Name };
System.assertEquals('Name', String.format('{0}', objectFormatArgs));
Map<String,Object> templateArgs = new Map<String,Object>{
  'name' => 'Ada',
  'count' => 2
};
System.assertEquals('Hello Ada: 2', 'Hello ${name}: ${count}'.template(templateArgs));
System.assertEquals('Escaped ${name} and Ada', 'Escaped $${name} and ${name}'.template(templateArgs));
Boolean missingTemplateValue = false;
try {
  'Hello ${missing}'.template(templateArgs);
} catch (StringException e) {
  missingTemplateValue = e.getMessage().contains('missing');
}
System.assert(missingTemplateValue);
System.assertEquals('a' + '\r\n' + 'b', 'a\r\nb');
String alphabet = 'abcdefghijklmnopqrstuvwxyz';
System.assertEquals('abcdefg...', alphabet.abbreviate(10));
System.assertEquals('...ijklmn...', alphabet.abbreviate(8, 12));
String machine = 'i am a machine';
System.assertEquals('robot', machine.difference('i am a robot'));
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
System.assertEquals(4, edge.indexOfAnyBut('abΩc'));
String overlaySource = 'abcdef';
System.assertEquals('abZZef', overlaySource.overlay('ZZ', 2, 4));
System.assertEquals('XXabcdef', overlaySource.overlay('XX', -2, 0));
String mixed = 'The Ω42';
System.assertEquals('tHE ω42', mixed.swapCase());
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
System.assertEquals('"<>&', escapedHtmlCore.unescapeXml());
String escapedXmlApos = '&apos;';
String xmlApos = escapedXmlApos.unescapeXml();
System.assertEquals(39, xmlApos.codePointAt(0));
String replacementEntity = '&#xFFFD;';
String replacementValue = replacementEntity.unescapeXml();
System.assertEquals(65533, replacementValue.codePointAt(0));
String invalidXmlNumeric = '&#0;&#x0;&#xD800;&#55296;&#x110000;&#+65;&#x+41;';
System.assertEquals(invalidXmlNumeric, invalidXmlNumeric.unescapeXml());
String replaceEmpty = 'abc';
System.assertEquals('abc', replaceEmpty.replace('', 'x'));
System.assertEquals('abc', replaceEmpty.remove(''));
System.assert(String.isBlank(null));
System.assert(!String.isNotBlank(null));
System.assert(String.isBlank('$RecordType.Name'));
System.assert(!String.isNotBlank('$RecordType.Name'));
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
System.assertEquals(62976, face.codePointAt(0));
System.assertEquals(62976, face.codePointBefore(1));
System.assertEquals(1, face.codePointCount(0, 1));
List<Integer> faceChars = face.getChars();
System.assertEquals(1, faceChars.size());
System.assertEquals(62976, faceChars.get(0));

System.assertEquals('abc...', 'abcdefg'.abbreviate(6));
System.assertEquals('...mnopq...', 'abcdefghijklmnopqrstuvwxyz'.abbreviate(12, 11));
System.assertEquals('abXXef', 'abcdef'.overlay('XX', 4, 2));
System.assertEquals('abcdefZZ', 'abcdef'.overlay('ZZ', 99, 100));
System.assertEquals('aZ 9ω', 'Az 9Ω'.swapCase());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringFormatTypedElementsAPI67(t *testing.T) {
	program, err := CompileAnonymous(`
List<Object> numberArgs = new List<Object>{ 2 };
Boolean caughtNumber = false;
try {
  String.format('{0,number}', numberArgs);
} catch (StringException e) {
  caughtNumber = true;
  System.assertEquals('Cannot format given Object as a Number', e.getMessage());
}
System.assert(caughtNumber);

List<Object> dateArgs = new List<Object>{ Datetime.newInstance(2024, 1, 2, 3, 4, 5) };
Boolean caughtDate = false;
try {
  String.format('{0,date,yyyy-MM-dd}', dateArgs);
} catch (StringException e) {
  caughtDate = true;
  System.assertEquals('Cannot format given Object (java.lang.String) as a Date', e.getMessage());
}
System.assert(caughtDate);

Boolean caughtChoice = false;
try {
  String.format('{0,choice,0#none|1#one|1<many}', numberArgs);
} catch (StringException e) {
  caughtChoice = true;
  System.assertEquals('''2'' is not a Number', e.getMessage());
}
System.assert(caughtChoice);

Boolean caughtNullChoice = false;
try {
  String.format('{0,choice,0#none|1#one|1<many}', new List<Object>{ null });
} catch (StringException e) {
  caughtNullChoice = true;
  System.assertEquals('''null'' is not a Number', e.getMessage());
}
System.assert(caughtNullChoice);

Boolean caughtUnknown = false;
try {
  String.format('{0,unknown}', numberArgs);
} catch (StringException e) {
  caughtUnknown = true;
  System.assertEquals('Unknown format type "unknown"', e.getMessage());
}
System.assert(caughtUnknown);

Boolean caughtUppercaseNumber = false;
try {
  String.format('{0,NUMBER}', numberArgs);
} catch (StringException e) {
  caughtUppercaseNumber = true;
  System.assertEquals('Cannot format given Object as a Number', e.getMessage());
}
System.assert(caughtUppercaseNumber);

System.assertEquals('Ada', String.format('{0}', new List<String>{ 'Ada' }));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringFormatMissingKnownTypeUsesUntypedPlaceholder(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('{3}', String.format('{3,number}', new List<Object>{ '2' }));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringFormatMissingUnknownTypeStillValidatesType(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
  String.format('{3,unknown}', new List<Object>{ '2' });
} catch (StringException e) {
  caught = true;
  System.assertEquals('Unknown format type "unknown"', e.getMessage());
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringFormatEmptyTypeUsesBadArgumentSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
  String.format('{0,}', new List<Object>{ '2' });
} catch (StringException e) {
  caught = true;
  System.assertEquals('Bad argument syntax: [at pattern index 1] "0,}"', e.getMessage());
}
System.assert(caught);
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
		{method: "swapCase", args: []Value{String("x")}},
		{method: "replace", args: []Value{Null, String("x")}},
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
	if _, err := stringStatic("String.format", []Value{String("{0,number,#.00}"), List(Int(42))}); err == nil || err.Error() != "Cannot format given Object as a Number" {
		t.Fatalf("String.format expected typed number StringException, got %v", err)
	}
	if _, err := stringStatic("String.format", []Value{String("{0"), List(Int(42))}); err == nil || !strings.Contains(err.Error(), "unmatched") {
		t.Fatalf("String.format expected unmatched brace error, got %v", err)
	}
	if _, err := stringStatic("String.fromCharArray", []Value{List(String("x"))}); err == nil {
		t.Fatal("String.fromCharArray expected bad argument error")
	}
	if _, err := stringStatic("String.getLevenshteinDistance", []Value{String("a"), String("b"), Int(-1)}); err == nil {
		t.Fatal("String.getLevenshteinDistance expected bad threshold error")
	}
	if _, _, err := callStringMember(String(`\u00ZZ`), "unescapeUnicode", nil); err == nil {
		t.Fatal("String.unescapeUnicode expected bad escape error")
	}
	if _, handled, err := callStringMember(String("${name}"), "template", []Value{String("bad")}); !handled || err == nil {
		t.Fatalf("String.template expected bad argument error, handled=%v err=%v", handled, err)
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
	quotedPattern, handled, err := callStringMember(String(`SELECT Id FROM Account WHERE Id = {!CurrentAccount.Id}`), "replaceAll", []Value{String(`(?i)\Q{!CurrentAccount.Id}\E`), String(`001000000000001`)})
	if err != nil || !handled || quotedPattern.Text != `SELECT Id FROM Account WHERE Id = 001000000000001` {
		t.Fatalf("replaceAll quoted pattern = %#v handled=%v err=%v", quotedPattern, handled, err)
	}
	quotedLookaround, handled, err := callStringMember(String(`{"type":"Account__c","field":"Name__c"}`), "replaceAll", []Value{String(`(?<=")([^"]*__c)(?=")`), String(`pkg1__$1`)})
	if err != nil || !handled || quotedLookaround.Text != `{"type":"pkg1__Account__c","field":"pkg1__Name__c"}` {
		t.Fatalf("replaceAll quoted lookaround = %#v handled=%v err=%v", quotedLookaround, handled, err)
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
	backrefSplit, handled, err := callStringMember(String("axa axb bxb"), "split", []Value{String(`([ab])x\1`), Int(-1)})
	if err != nil || !handled || backrefSplit.Kind != ValueList || len(backrefSplit.List) != 3 || backrefSplit.List[0].Text != "" || backrefSplit.List[1].Text != " axb " || backrefSplit.List[2].Text != "" {
		t.Fatalf("split backreference = %#v handled=%v err=%v", backrefSplit, handled, err)
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

func TestExecPatternMatchesSupportsLeadingPositiveLookaheads(t *testing.T) {
	program, err := CompileAnonymous(`
String alphaNumeric = '(?=.*\\d)(?=.*[a-zA-Z]).*';
String caseNumeric = '(?=.*\\d)(?=.*[a-z])(?=.*[A-Z]).*';
System.assert(Pattern.matches(alphaNumeric, 'a1a1a1a1'));
System.assert(!Pattern.matches(alphaNumeric, 'aaaaaaaa'));
System.assert(!Pattern.matches(alphaNumeric, '11111111'));
System.assert(Pattern.matches(caseNumeric, 'a1A1a1A1'));
System.assert(!Pattern.matches(caseNumeric, 'a1a1a1a1'));
System.assert(!Pattern.matches(caseNumeric, 'A1A1A1A1'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternMatchesSupportsFixedCountPossessiveQuantifier(t *testing.T) {
	program, err := CompileAnonymous(`
String settledDatePattern = '$|^[0-9]{14}+$';
System.assert(Pattern.matches(settledDatePattern, '20240101123456'));
System.assert(Pattern.matches(settledDatePattern, ''));
System.assert(!Pattern.matches(settledDatePattern, '20240101'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPatternMatchesSupportsVariablePossessiveQuantifier(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{pattern: "a++a", input: "aa", want: false},
		{pattern: "a*+", input: "aaa", want: true},
		{pattern: "a?+", input: "a", want: true},
		{pattern: "a?+a", input: "a", want: false},
		{pattern: "a{1,3}+", input: "aaa", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.pattern, func(t *testing.T) {
			got, err := patternMatches([]Value{String(tc.pattern), String(tc.input)})
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != ValueBool || got.Bool != tc.want {
				compiled, compileErr := compileRegexp2Source("Pattern.matches", tc.pattern)
				t.Fatalf("Pattern.matches(%q, %q) compiled %q err %v = %#v, want %v", tc.pattern, tc.input, compiled, compileErr, got, tc.want)
			}
		})
	}
}

func TestStringRegexSplitRejectsNullableEdges(t *testing.T) {
	split, handled, err := callStringMember(String("axa axb bxb"), "split", []Value{String(`([ab])x\1`), Int(-1)})
	if err != nil || !handled || split.Kind != ValueList || len(split.List) != 3 || split.List[0].Text != "" || split.List[1].Text != " axb " || split.List[2].Text != "" {
		t.Fatalf("backreference split = %#v handled=%v err=%v", split, handled, err)
	}
	defaultSplit, handled, err := callStringMember(String("axa axb bxb"), "split", []Value{String(`([ab])x\1`)})
	if err != nil || !handled || defaultSplit.Kind != ValueList || len(defaultSplit.List) != 2 || defaultSplit.List[0].Text != "" || defaultSplit.List[1].Text != " axb " {
		t.Fatalf("default backreference split = %#v handled=%v err=%v", defaultSplit, handled, err)
	}
	nullable, handled, err := callStringMember(String("ab cd"), "split", []Value{String("a*"), Int(-1)})
	if err != nil || !handled || nullable.Kind != ValueList || len(nullable.List) != 7 || nullable.List[0].Text != "" || nullable.List[1].Text != "" || nullable.List[2].Text != "b" || nullable.List[6].Text != "" {
		t.Fatalf("nullable split = %#v handled=%v err=%v", nullable, handled, err)
	}
	anchorStart, handled, err := callStringMember(String("abc"), "split", []Value{String("^"), Int(-1)})
	if err != nil || !handled || anchorStart.Kind != ValueList || len(anchorStart.List) != 1 || anchorStart.List[0].Text != "abc" {
		t.Fatalf("anchor start split = %#v handled=%v err=%v", anchorStart, handled, err)
	}
	anchorEnd, handled, err := callStringMember(String("abc"), "split", []Value{String("$"), Int(-1)})
	if err != nil || !handled || anchorEnd.Kind != ValueList || len(anchorEnd.List) != 2 || anchorEnd.List[0].Text != "abc" || anchorEnd.List[1].Text != "" {
		t.Fatalf("anchor end split = %#v handled=%v err=%v", anchorEnd, handled, err)
	}
	charSplit, handled, err := callStringMember(String("abc"), "split", []Value{String(""), Int(-1)})
	if err != nil || !handled || charSplit.Kind != ValueList || len(charSplit.List) != 4 || charSplit.List[0].Text != "a" || charSplit.List[3].Text != "" {
		t.Fatalf("empty regex split = %#v handled=%v err=%v", charSplit, handled, err)
	}
	split, handled, err = callStringMember(String(""), "split", []Value{String("x")})
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
	System.assertEquals('Blob[5]', String.valueOf(hello));
	System.assertEquals(5, hello.size());
System.assertEquals('68656c6c6f', EncodingUtil.convertToHex(hello));
System.assertEquals('68656c6c6f', EncodingUtil.ConvertToHex(hello));
System.assertEquals('68656c6c6f', EncodingUtil.ConvertTohex(hello));
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
Id recordId = '001000000000001AAA';
System.assertEquals('001000000000001AAA', EncodingUtil.urlEncode(recordId, 'UTF-8'));
Blob md5 = Crypto.generateDigest('MD5', hello);
Blob sha1 = Crypto.generateDigest('SHA1', hello);
Blob sha256 = Crypto.generateDigest('SHA-256', hello);
Blob sha256NoDash = Crypto.generateDigest('sha256', hello);
Blob sha384 = Crypto.generateDigest('SHA-384', hello);
Blob sha512 = Crypto.generateDigest('SHA-512', hello);
Blob sha3 = Crypto.generateDigest('SHA3-256', hello);
System.assertEquals('5d41402abc4b2a76b9719d911017c592', EncodingUtil.convertToHex(md5));
System.assertEquals('aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d', EncodingUtil.convertToHex(sha1));
System.assertEquals('2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824', EncodingUtil.convertToHex(sha256));
System.assertEquals(EncodingUtil.convertToHex(sha256), EncodingUtil.convertToHex(sha256NoDash));
System.assertEquals('59e1748777448c69de6b800d7a33bbfb9ff1b463e44354c3553bcdb9c666fa90125a3c79f90397bdf5f6a13de828684f', EncodingUtil.convertToHex(sha384));
System.assertEquals('9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043', EncodingUtil.convertToHex(sha512));
System.assertEquals('3338be694f50c5f338814986cdf0686453a888b84f424d792af4b9202398f392', EncodingUtil.convertToHex(sha3));
System.assertEquals('snow+trail', EncodingUtil.urlEncode('snow trail', ' utf_8 '));
System.assertEquals('A B+Ω', EncodingUtil.urlDecode('A+B%2B%CE%A9', 'UTF8'));
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
	Blob aes128 = Crypto.generateAESKey(128);
	Blob aes192 = Crypto.generateAESKey(192);
	Blob aes256 = Crypto.generateAESKey(256);
	System.assertEquals('Blob[16]', String.valueOf(aes128));
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

func TestExecCryptoAreEqualConstantTimeRejectsAbsentSalesforceAPI(t *testing.T) {
	program, err := CompileAnonymous(`
Crypto.areEqualConstantTime(Blob.valueOf('hello'), Blob.valueOf('hello'));
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), `unsupported call "Crypto.areEqualConstantTime local key, certificate, encryption, and random surfaces"`) {
		t.Fatalf("err = %v, want Salesforce-shaped unsupported API rejection", err)
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
Blob cbc = Crypto.encrypt('AES256-CBC', key, iv, Blob.valueOf('hello'));
System.assertEquals(EncodingUtil.convertToHex(encrypted), EncodingUtil.convertToHex(cbc));
System.assertEquals('hello', Crypto.decrypt('AES256-CBC', key, iv, cbc).toString());
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
Blob cbc = Crypto.encryptWithManagedIV('AES256-CBC', key, Blob.valueOf('hello'));
System.assertEquals(32, cbc.size());
System.assertEquals('hello', Crypto.decryptWithManagedIV('AES256-CBC', key, cbc).toString());
Blob gcm = Crypto.encryptWithManagedIV('AES256-GCM', key, Blob.valueOf('hello'), Blob.valueOf('aad'));
System.assertEquals(34, gcm.size());
System.assertEquals('hello', Crypto.decryptWithManagedIV('AES256-GCM', key, gcm, Blob.valueOf('aad')).toString());
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

func TestExecCompressionZipRoundTrip(t *testing.T) {
	program, err := CompileAnonymous(`
compression.ZipWriter writer = new compression.ZipWriter();
compression.ZipEntry first = writer.addEntry('a.txt', Blob.valueOf('alpha'));
first.setComment('first file');
writer.addEntry('b.txt', 'second file', Datetime.now(), compression.Method.STORED, Blob.valueOf('bravo'));
System.assertEquals(2, writer.getEntries().size());
System.assertEquals('a.txt', writer.getEntry('a.txt').getName());
System.assertEquals(true, writer.getEntryNames().contains('b.txt'));
System.assertEquals(compression.Level.DEFAULT_LEVEL, writer.getLevel());
writer.setLevel(compression.Level.BEST_SPEED);
writer.setMethod(compression.Method.STORED);
System.assertEquals(compression.Level.BEST_SPEED, writer.getLevel());
System.assertEquals(compression.Method.STORED, writer.getMethod());
Blob archive = writer.getArchive();
compression.ZipReader reader = new compression.ZipReader(archive);
System.assertEquals(2, reader.getEntries().size());
System.assertEquals('alpha', reader.extract('a.txt').toString());
System.assertEquals('bravo', reader.extract(reader.getEntry('b.txt')).toString());
System.assertEquals(true, reader.getEntriesMap().containsKey('a.txt'));
System.assertEquals('a.txt', reader.getEntryNames().get(0));
writer.removeEntry('a.txt');
System.assertEquals(null, writer.getEntry('a.txt'));
System.assertEquals(1, writer.getEntries().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecWaveQueryBuilderLocalBuild(t *testing.T) {
	program, err := CompileAnonymous(`
wave.ProjectionNode amount = wave.QueryBuilder.get('Amount').sum().alias('total');
System.assertEquals('sum(Amount) as total', amount.build());
wave.QueryNode query = wave.QueryBuilder.load('dataset', 'v1')
	.filter('Amount > 0')
	.group()
	.foreach(new List<wave.ProjectionNode>{amount})
	.cap(10);
String built = query.build('q');
System.assert(built.contains('q'));
System.assert(built.contains('load'));
System.assert(built.contains('dataset'));
System.assert(built.contains('filter(Amount > 0)'));
System.assert(built.contains('group()'));
System.assert(built.contains('foreach'));
System.assert(built.contains('cap(10)'));
wave.QueryNode byName = wave.QueryBuilder.loadByDeveloperName('pkg.Dataset').filter(new List<String>{'A == 1', 'B == 2'});
System.assert(byName.build('named').contains('pkg.Dataset'));
System.assert(wave.QueryBuilder.union(new List<wave.QueryNode>{query, byName}).build('u').contains('union'));
System.assert(wave.QueryBuilder.cogroup(new List<wave.QueryNode>{query}, new List<List<String>>{new List<String>{'AccountId'}}).build('c').contains('cogroup'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecWaveQueryExecuteReturnsEmptyLiteralJson(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.LiteralJson result = wave.QueryBuilder.load('dataset', 'v1').execute('q');
System.assertNotEquals(null, result);
List<Object> rows = (List<Object>) result.json;
System.assertEquals(0, rows.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecContextAndOrgInstrumentationLocalSurfaces(t *testing.T) {
	if !generatedPlatformRuntimePrecedenceType("Context.IndustriesContext") {
		t.Fatal("Context.IndustriesContext must use platform runtime before generated methods")
	}
	program, err := CompileAnonymous(`
Context.IndustriesContext context = new Context.IndustriesContext();
Context.IndustriesContext clonedContext = (Context.IndustriesContext)context.clone();
System.assertNotEquals(null, clonedContext);
Map<String,Object> input = new Map<String,Object>{'recordId' => '001000000000001'};
System.assertEquals('001000000000001', context.buildContext(input).get('recordId'));
System.assertEquals('001000000000001', context.getContext(input).get('recordId'));
System.assertEquals('001000000000001', context.queryTags(input).get('recordId'));
context.deleteContext(input);
context.evictContextDefinition(input);
OrgInstrumentationOperation op = new OrgInstrumentationOperation();
OrgInstrumentationContext metricContext = op.start(OrgMetricPublishTypeEnum.REQUEST_COUNT);
System.assertNotEquals(null, metricContext);
System.assertEquals(OrgMetricPublishTypeEnum.REQUEST_COUNT, metricContext.getPublishType());
metricContext.startTime();
System.assertEquals(0L, metricContext.getDuration());
metricContext.end();
System.assertNotEquals(null, op.createNewSpan());
op.setMetricTags(new Map<String,String>{'route' => 'local'});
op.publishCustomIncrementalValue('local.metric', 1L);
op.publishCustomHistogramValues('local.metric', 2L);
op.publishCustomPercentileSet('local.metric', 3L);
op.publishRequestCountAndDuration(1L, 200, 10L);
op.publishRequestCountAndDuration(1L, 200, 10L, 'local.metric');
op.publishIncrementalValue(OrgMetricTypeEnum.REQUEST_COUNT, 1L, 200);
op.publishPercentileSet(OrgMetricTypeEnum.REQUEST_COUNT, 4L);
op.end(metricContext);
op.endWithStatus(metricContext, 200);
System.assertNotEquals(null, OrgMetricServiceEnum.CRM);
System.assertNotEquals(null, OrgMetricServiceEnum.GUS);
System.assertNotEquals(null, OrgMetricServiceEnum.IDXR);
System.assertNotEquals(null, OrgMetricServiceEnum.LEGO);
System.assertNotEquals(null, OrgMetricServiceEnum.NetZeroMarketplace);
System.assertNotEquals(null, OrgMetricServiceEnum.TBID);
System.assertNotEquals(null, OrgMetricServiceEnum.UOE);
System.assertNotEquals(null, OrgMetricServiceEnum.WorkDotCom);
OrgInstrumentationService service = new OrgInstrumentationService();
Map<String,String> tags = new Map<String,String>{'route' => 'local'};
System.assertNotEquals(null, service.getInstrumentationOperation('local', tags));
System.assertNotEquals(null, service.getInstrumentationOperation('local', tags, new List<Double>{0.5, 0.95}));
System.assertNotEquals(null, service.getInstrumentationOperation('local', tags, new List<Long>{1L, 10L}));
System.assertEquals(0, service.getTracerContext().size());
HttpRequest req = new HttpRequest();
service.propagateContext(req);
System.assertEquals('local', req.getHeader('x-glade-instrumentation-context'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserProvisioningBatchableLocalDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
UserProvisioning.ProvisioningBatchable batchable = new UserProvisioning.ProvisioningBatchable(new List<SObject>());
Map<String,Object> input = new Map<String,Object>{'uprId' => '0PR000000000001'};
System.assertEquals('0PR000000000001', batchable.flowInputPreprocessing(input).get('uprId'));
System.assertEquals('', batchable.getFlowName());
System.assertEquals('', batchable.getFlowNamespace());
System.assertEquals('', batchable.getEventPrefix());
System.assertEquals(false, batchable.hasFlow());
System.assertEquals(false, batchable.hasFlowOrApex());
System.assertEquals(0, batchable.getPerBatchUPL().size());
System.assertEquals(0, batchable.getPerBatchUPR().size());
System.assertEquals(0, batchable.getUprToNewUplMap().size());
Database.QueryLocator locator = batchable.start(null);
System.assertEquals('', locator.getQuery());
batchable.execute(null, new List<UserProvisioningRequest>());
batchable.finish(null);
batchable.postBatchProcessing();
UserProvisioning.LinkingBatchable linking = new UserProvisioning.LinkingBatchable('0PR000000000001');
System.assertEquals(false, linking.hasFlowOrApex());
System.assertEquals('', linking.start(null).getQuery());
UserProvisioning.CommittingBatchable committing = new UserProvisioning.CommittingBatchable();
System.assertEquals('', committing.start(null).getQuery());
committing.execute(null, new List<SObject>());
committing.finish(null);
UserProvisioning.DeletingBatchable deleting = new UserProvisioning.DeletingBatchable();
System.assertEquals('', deleting.start(null).getQuery());
deleting.execute(null, new List<SObject>());
deleting.finish(null);
UserProvisioning.RequestingBatchable requesting = new UserProvisioning.RequestingBatchable();
System.assertEquals('', requesting.start(null).getQuery());
requesting.execute(null, new List<UserProvisioningRequest>());
requesting.finish(null);
UserProvisioning.UPASCleaningBatchable cleaning = new UserProvisioning.UPASCleaningBatchable();
System.assertEquals('', cleaning.start(null).getQuery());
cleaning.execute(null, new List<SObject>());
cleaning.finish(null);
UserProvisioning.FlowProvisionBase flowBase = new UserProvisioning.FlowProvisionBase();
System.assertEquals('', flowBase.getFlowName());
System.assertEquals('', flowBase.getFlowNamespace());
System.assertEquals(false, flowBase.hasFlow());
System.assertEquals(false, flowBase.hasFlowOrApex());
System.assert(new UserProvisioning.UserProvisioningProcessHandler().invoke(null) != null);
System.assert(new UserProvisioning.DummyConnectorApexHandler().invoke(null) != null);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserProvisioningBatchableLifecycleFixture(t *testing.T) {
	program, err := CompileAnonymous(`
Database.BatchableContextImpl bc = new Database.BatchableContextImpl();
UserProvisioning.ProvisioningProcessHandlerOutput output = new UserProvisioning.ProvisioningProcessHandlerOutput();
Map<String,Object> values = new Map<String,Object>();
UserProvisioning.CollectingBatchable collectingBatchable = new UserProvisioning.CollectingBatchable('0','upr','app');
System.assertNotEquals(null, collectingBatchable.clone());
System.assertNotEquals(null, collectingBatchable.start(bc));
collectingBatchable.execute(bc, new List<UserProvisioningRequest>());
collectingBatchable.finish(bc);
System.assertNotEquals(null, collectingBatchable.flowInputPreprocessing(values));
collectingBatchable.flowPostProcessing(output, new Account(Name = 'CollectingBatchable'));
System.assertNotEquals(null, collectingBatchable.getEventPrefix());
System.assertNotEquals(null, collectingBatchable.getFlowName());
System.assertNotEquals(null, collectingBatchable.getFlowNamespace());
System.assertNotEquals(null, collectingBatchable.getPerBatchUPL());
System.assertNotEquals(null, collectingBatchable.getPerBatchUPR());
System.assertNotEquals(null, collectingBatchable.getUprToNewUplMap());
System.assertEquals(false, collectingBatchable.hasFlow());
System.assertEquals(false, collectingBatchable.hasFlowOrApex());
collectingBatchable.postBatchProcessing();
UserProvisioning.CommittingBatchable committingBatchable = new UserProvisioning.CommittingBatchable('upr');
System.assertNotEquals(null, committingBatchable.clone());
System.assertNotEquals(null, committingBatchable.start(bc));
committingBatchable.execute(bc, new List<SObject>());
committingBatchable.finish(bc);
UserProvisioning.DeletingBatchable deletingBatchable = new UserProvisioning.DeletingBatchable('upr');
System.assertNotEquals(null, deletingBatchable.clone());
System.assertNotEquals(null, deletingBatchable.start(bc));
deletingBatchable.execute(bc, new List<SObject>());
deletingBatchable.finish(bc);
UserProvisioning.LinkingBatchable linkingBatchable = new UserProvisioning.LinkingBatchable('upr');
System.assertNotEquals(null, linkingBatchable.clone());
System.assertNotEquals(null, linkingBatchable.start(bc));
linkingBatchable.execute(bc, new List<SObject>());
linkingBatchable.finish(bc);
System.assertNotEquals(null, linkingBatchable.getFlowName());
System.assertNotEquals(null, linkingBatchable.getFlowNamespace());
System.assertEquals(false, linkingBatchable.hasFlow());
System.assertEquals(false, linkingBatchable.hasFlowOrApex());
UserProvisioning.PluginBatchable pluginBatchable = new UserProvisioning.PluginBatchable(new List<SObject>());
System.assertNotEquals(null, pluginBatchable.clone());
System.assertNotEquals(null, pluginBatchable.start(bc));
pluginBatchable.execute(bc, new List<UserProvisioningRequest>());
System.assertNotEquals(null, pluginBatchable.flowInputPreprocessing(values));
pluginBatchable.flowPostProcessing(output, new Account(Name = 'PluginBatchable'));
System.assertNotEquals(null, pluginBatchable.getEventPrefix());
System.assertNotEquals(null, pluginBatchable.getFlowName());
System.assertNotEquals(null, pluginBatchable.getFlowNamespace());
System.assertNotEquals(null, pluginBatchable.getPerBatchUPL());
System.assertNotEquals(null, pluginBatchable.getPerBatchUPR());
System.assertNotEquals(null, pluginBatchable.getUprToNewUplMap());
System.assertEquals(false, pluginBatchable.hasFlow());
System.assertEquals(false, pluginBatchable.hasFlowOrApex());
pluginBatchable.postBatchProcessing();
UserProvisioning.ProvisioningBatchable provisioningBatchable = new UserProvisioning.ProvisioningBatchable(new List<SObject>());
System.assertNotEquals(null, provisioningBatchable.clone());
System.assertNotEquals(null, provisioningBatchable.start(bc));
provisioningBatchable.execute(bc, new List<UserProvisioningRequest>());
provisioningBatchable.finish(bc);
System.assertNotEquals(null, provisioningBatchable.flowInputPreprocessing(values));
provisioningBatchable.flowPostProcessing(output, new Account(Name = 'ProvisioningBatchable'));
System.assertNotEquals(null, provisioningBatchable.getEventPrefix());
System.assertNotEquals(null, provisioningBatchable.getFlowName());
System.assertNotEquals(null, provisioningBatchable.getFlowNamespace());
System.assertNotEquals(null, provisioningBatchable.getPerBatchUPL());
System.assertNotEquals(null, provisioningBatchable.getPerBatchUPR());
System.assertNotEquals(null, provisioningBatchable.getUprToNewUplMap());
System.assertEquals(false, provisioningBatchable.hasFlow());
System.assertEquals(false, provisioningBatchable.hasFlowOrApex());
provisioningBatchable.postBatchProcessing();
UserProvisioning.RequestingBatchable requestingBatchable = new UserProvisioning.RequestingBatchable(new List<SObject>());
System.assertNotEquals(null, requestingBatchable.clone());
System.assertNotEquals(null, requestingBatchable.start(bc));
requestingBatchable.execute(bc, new List<UserProvisioningRequest>());
requestingBatchable.finish(bc);
UserProvisioning.UPASCleaningBatchable uPASCleaningBatchable = new UserProvisioning.UPASCleaningBatchable('upr');
System.assertNotEquals(null, uPASCleaningBatchable.clone());
System.assertNotEquals(null, uPASCleaningBatchable.start(bc));
uPASCleaningBatchable.execute(bc, new List<SObject>());
uPASCleaningBatchable.finish(bc);
Test.startTest();
Database.executeBatch(new UserProvisioning.CommittingBatchable('upr'), 200);
Database.executeBatch(new UserProvisioning.DeletingBatchable('upr'), 200);
Database.executeBatch(new UserProvisioning.LinkingBatchable('upr'), 200);
Database.executeBatch(new UserProvisioning.RequestingBatchable(new List<SObject>()), 200);
Database.executeBatch(new UserProvisioning.UPASCleaningBatchable('upr'), 200);
Test.stopTest();
System.assertEquals(5, [SELECT COUNT() FROM AsyncApexJob WHERE JobType = 'BatchApex' AND Status = 'Completed']);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCompileUserProvisioningPluginBaseRejectsConstruction(t *testing.T) {
	program, err := CompileAnonymous(`
UserProvisioning.UserProvisioningPlugin plugin = new UserProvisioning.UserProvisioningPlugin();
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(nil).Execute(program)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "abstract") {
		t.Fatalf("execute error = %v, want abstract base-class construction rejection", err)
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
System.assert(valid.equals('001B000001DVM9t'));
System.assert(valid.equals('001B000001DVM9tIAH'));
System.assertEquals('001B000001DVM9t', valid.toString());
System.assertEquals('001B000001DVM9tIAH', String.valueOf(valid));
System.assertEquals('id=001B000001DVM9tIAH', 'id=' + valid);
System.assertEquals('Account', Id.valueOf('001B000001DVM9t').getSObjectType().getDescribe().getName());
System.assertEquals('001B000001DVM9t', valid.to15());
Id longId = Id.valueOf('001B000001DVM9tIAH');
System.assertEquals('001B000001DVM9t', longId.to15());
System.assertEquals('001B000001DVM9tIAH', restored.toString());
System.assertEquals('001B000001DVM9tIAH', restoredLowerChecksum.toString());
List<String> ids = new List<String>{valid};
System.assertEquals('001B000001DVM9tIAH', ids[0]);

String text = 'trail';
System.assert(text.equals('trail'));
System.assert(!text.equals('ridge'));
System.assert('001B000001DVM9t'.equals(valid));
System.assert('001B000001DVM9tIAH'.equals(valid));
String typedIdText = valid;
System.assert(typedIdText.equals('001B000001DVM9tIAH'));
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
System.assertEquals('(1)', left.toString());

URL base = URL.getOrgDomainUrl();
System.assertEquals('https://local.glade.example', base.toExternalForm());
System.assertEquals('https://local.glade.example/servlet/servlet.FileDownload?field=Logo__c&id=001B000001DVM9t', URL.getFileFieldURL('001B000001DVM9t', 'Logo__c'));
System.assertEquals('https', base.getProtocol());
System.assertEquals('local.glade.example', base.getHost());
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
try {
    System.hashCode(null);
    System.assert(false, 'System.hashCode(null) should fail');
} catch (Exception e) {
    System.assertEquals('System.NullPointerException', e.getTypeName());
    System.assertEquals('Argument 1 cannot be null', e.getMessage());
}
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
System.assertEquals('2026-05-02', String.valueOf(today), 'System.today should use the VM clock');
System.assertEquals(0, Date.newInstance(2026, 5, 1).monthsBetween(Date.newInstance(2026, 5, 31)));
System.assertEquals(1, Date.newInstance(2026, 5, 31).monthsBetween(Date.newInstance(2026, 6, 1)));
System.assertEquals(-12, Date.newInstance(2026, 5, 1).monthsBetween(Date.newInstance(2025, 5, 1)));
System.assertEquals(5, today.Month());
Datetime now = System.now();
System.assertEquals('2026-05-02 12:00:00', now.formatGmt('yyyy-MM-dd HH:mm:ss'));
System.assertEquals('6', now.formatGmt('u'));
System.assertEquals('18', now.formatGmt('w'));
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
System.assertEquals(2, constructed.getLineNumber());
System.assertEquals('AnonymousBlock: line 2, column 1', constructed.getStackTraceString());
System.assertEquals('System.DmlException: blocked', constructed.toString());
Exception noMessage = new DmlException();
System.assertEquals('Script-thrown exception', noMessage.getMessage());
Exception systemPrefixed = new System.DmlException('system blocked');
System.assertEquals('System.DmlException', systemPrefixed.getTypeName());
System.assertEquals('System.DmlException: system blocked', systemPrefixed.toString());
Exception allCapsDML = new DMLException('caps blocked');
System.assertEquals('System.DMLException', allCapsDML.getTypeName());
	Exception aura = new AuraHandledException('aura blocked');
	System.assertEquals('System.AuraHandledException', aura.getTypeName());
	System.assertEquals('aura blocked', aura.getMessage());
	try {
		throw aura;
	} catch (Exception e) {
		System.assertEquals('Script-thrown exception', e.getMessage());
	}
	AuraHandledException explicitAura = new AuraHandledException('constructor hidden');
	explicitAura.setMessage('explicit aura message');
	try {
		throw explicitAura;
	} catch (AuraHandledException e) {
		System.assertEquals('explicit aura message', e.getMessage());
	}

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
Exception wrapperError = new DmlException('outer');
System.assertEquals(null, wrapperError.getCause());
Exception cause = new QueryException('root cause');
wrapperError.initCause(cause);
Exception recovered = wrapperError.getCause();
System.assertEquals('System.QueryException', recovered.getTypeName());
System.assertEquals('root cause', recovered.getMessage());
Exception constructedCause = new DmlException('wrapped', cause);
System.assertEquals('wrapped', constructedCause.getMessage());
System.assertEquals('root cause', constructedCause.getCause().getMessage());

Boolean repeatCaught = false;
try {
	wrapperError.initCause(null);
} catch (Exception e) {
	repeatCaught = true;
	System.assertEquals('System.IllegalStateException', e.getTypeName());
	System.assertEquals('Can''t overwrite cause', e.getMessage());
}
System.assert(repeatCaught, 'repeat initCause should throw');
System.assertEquals('root cause', wrapperError.getCause().getMessage());

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

func TestExecJSONExceptionKeepsSpecificUnsupportedExceptionMembers(t *testing.T) {
	program, err := CompileAnonymous(`
Exception e = new JSONException('bad json');

Boolean fieldsCaught = false;
try {
	e.getInaccessibleFields();
} catch (Exception ex) {
	fieldsCaught = true;
	System.assertEquals('System.TypeException', ex.getTypeName());
}
System.assert(fieldsCaught, 'JSONException.getInaccessibleFields should throw');

Boolean causeCaught = false;
try {
	e.initCause(new RootCauseException('root'));
} catch (Exception ex) {
	causeCaught = true;
	System.assertEquals('System.NullPointerException', ex.getTypeName());
}
System.assert(causeCaught, 'JSONException.initCause should throw');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	registerCustomException(t, machine, "RootCauseException")
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomExceptionInheritsCoreConstructors(t *testing.T) {
	program, err := CompileAnonymous(`
Exception cause = new QueryException('root');
AppException wrapped = new AppException('wrapped', cause);
System.assertEquals('wrapped', wrapped.getMessage());
System.assertEquals('root', wrapped.getCause().getMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AppException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomExceptionCanCallSuperSetMessage(t *testing.T) {
	setProgram, err := CompileAnonymous("super.setMessage(message);")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
AppException ex = new AppException();
System.assertEquals('blocked', ex.getMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "AppException",
		SuperClass: "Exception",
		Constructors: []Method{{
			Name:          "AppException",
			ClassName:     "AppException",
			Params:        []Param{{Name: "message", Type: "String"}},
			Program:       setProgram,
			IsConstructor: true,
		}},
		Fields: map[string]Field{
			"message": {Name: "message", Type: "String", InitialValue: String("blocked")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedNestedExceptionGetTypeName(t *testing.T) {
	program, err := CompileAnonymous(`
Exception e = new pkg.Outer.InnerException('blocked');
System.assertEquals('pkg.Outer.InnerException', e.getTypeName());
System.assertEquals('pkg.Outer.InnerException', pkg.Outer.InnerException.class.getName());
System.assertEquals('blocked', e.getMessage());
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer.InnerException", Namespace: "pkg", SuperClass: "Exception", Access: "global"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCoreBuiltinExceptionMatrix(t *testing.T) {
	exceptionNames := []string{
		"AssertException",
		"AsyncException",
		"BigObjectException",
		"CanvasException",
		"CalloutException",
		"DataWeaveScriptException",
		"DmlException",
		"DuplicateMessageException",
		"EmailException",
		"EmailTemplateRenderException",
		"EventObjectException",
		"ExternalObjectException",
		"FatalCursorException",
		"FinalException",
		"FlowException",
		"FormulaEvaluationException",
		"FormulaValidationException",
		"HandledException",
		"IllegalArgumentException",
		"IllegalStateException",
		"InvalidHeaderException",
		"InvalidParameterValueException",
		"InvalidReadOnlyUserDmlException",
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
		"TouchHandledException",
		"TypeException",
		"UnexpectedException",
		"UnsupportedOperationException",
		"VisualforceException",
		"XmlException",
	}
	runtimeProbes := map[string]struct {
		call    string
		message string
	}{
		"InvalidParameterValueException": {
			call:    "Auth.AuthToken.getAccessToken('provider', 'local');",
			message: "Invalid ID",
		},
		"NoAccessException": {
			call:    "Auth.JWT parsed = Auth.JWTUtil.parseJWTFromStringWithoutValidation('eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJwYXJzZWQtaXNzdWUiLCJzdWIiOiJwYXJzZWQtc3ViaiIsImF1ZCI6InBhcnNlZC1hdWQiLCJyb2xlcyI6WyJhZG1pbiIsInVzZXIiXSwibmJmIjoxMjMsImV4cCI6NDU2fQ.c2lnbmF0dXJl'); parsed.getNbfClockSkew();",
			message: "method is not available for a parsed JWT",
		},
		"NoDataFoundException": {
			call:    "Crypto.signWithCertificate('RSA-SHA999', Blob.valueOf('data'), 'cert');",
			message: `unsupported signature algorithm "RSA-SHA999"`,
		},
		"NullPointerException": {
			call:    "Date.valueOf();",
			message: "Date.valueOf expects String",
		},
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
		if probe, ok := runtimeProbes[name]; ok {
			source.WriteString(" = null;\ntry {\n")
			source.WriteString(probe.call)
			source.WriteString("\nSystem.assert(false, 'expected ")
			source.WriteString(name)
			source.WriteString(" runtime exception');\n} catch (Exception caught")
			source.WriteString(string(rune('A' + i/26)))
			source.WriteString(string(rune('A' + i%26)))
			source.WriteString(") { e")
			source.WriteString(string(rune('A' + i/26)))
			source.WriteString(string(rune('A' + i%26)))
			source.WriteString(" = caught")
			source.WriteString(string(rune('A' + i/26)))
			source.WriteString(string(rune('A' + i%26)))
			source.WriteString("; }\n")
		} else {
			source.WriteString(" = new ")
			source.WriteString(name)
			source.WriteString("('")
			source.WriteString(name)
			source.WriteString(" message');\n")
		}
		source.WriteString("System.assertEquals('System.")
		source.WriteString(name)
		source.WriteString("', e")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString(".getTypeName());\n")
		source.WriteString("System.assertEquals('System.")
		source.WriteString(name)
		source.WriteString(": ")
		message := name + " message"
		if probe, ok := runtimeProbes[name]; ok {
			message = probe.message
		} else if name == "TouchHandledException" {
			// Salesforce reserves this exception for Visualforce/Aura throws;
			// the local constructor contract uses the platform default message.
			message = "Script-thrown exception"
		}
		source.WriteString(message)
		source.WriteString("', e")
		source.WriteString(string(rune('A' + i/26)))
		source.WriteString(string(rune('A' + i%26)))
		source.WriteString(".toString());\n")
		if name == "QueryException" {
			source.WriteString("System.assertEquals(0, e")
			source.WriteString(string(rune('A' + i/26)))
			source.WriteString(string(rune('A' + i%26)))
			source.WriteString(".getInaccessibleFields().size());\n")
		}
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
System.assertEquals(null, Type.forName('System', 'Exception'));
System.assertNotEquals(null, Type.forName('System', 'DmlException'));

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
		{source: "EncodingUtil.convertFromHex('abc');", want: "invalid hexadecimal string"},
		{source: "EncodingUtil.convertFromHex('zz');", want: "invalid hexadecimal string"},
		{source: "Blob bad = EncodingUtil.convertFromHex('80'); bad.toString();", want: "Blob.toString invalid UTF-8 data"},
		{source: "EncodingUtil.urlEncode(null, 'UTF-8');", want: `Argument cannot be null.`},
		{source: "EncodingUtil.urlDecode('%zz', 'UTF-8');", want: "invalid URL escape"},
		{source: "Crypto.generateDigest('SHA-999', Blob.valueOf('x'));", want: `SHA-999 MessageDigest not available`},
		{source: "Crypto.generateDigest(' sha_256 ', Blob.valueOf('x'));", want: ` sha_256  MessageDigest not available`},
		{source: "Crypto.generateDigest('SHA3_256', Blob.valueOf('x'));", want: `SHA3_256 MessageDigest not available`},
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
			want: "cipherText must include managed IV",
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
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.want)
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

func TestExecTypeNewInstanceRunsFactoryTargetZeroArgConstructor(t *testing.T) {
	constructorProgram, err := CompileAnonymous(`marker = 'constructed';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
FactoryTarget made = (FactoryTarget)Type.forName('FactoryTarget').newInstance();
System.assertEquals('constructed', made.marker);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "FactoryTarget",
		Fields: map[string]Field{
			"marker": {Name: "marker", Type: "String"},
		},
		FieldOrder: []string{"marker"},
		Constructors: []Method{
			{Name: "FactoryTarget.<init>", ClassName: "FactoryTarget", Program: constructorProgram},
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
System.assertEquals(6, LoggingLevel.INFO.ordinal());
System.assertEquals('INFO', System.LoggingLevel.INFO.name());
System.debug(System.LoggingLevel.INFO, 'system qualified level');
System.LoggingLevel qualifiedLevel = System.LoggingLevel.WARN;
System.assertEquals('WARN', qualifiedLevel.name());
System.assertEquals(7, qualifiedLevel.ordinal());
System.debug(qualifiedLevel, 'qualified variable level');
List<LoggingLevel> levels = LoggingLevel.values();
System.assertEquals(9, levels.size());
LoggingLevel firstLevel = levels.get(0);
LoggingLevel lastLevel = levels.get(8);
System.assertEquals('NONE', firstLevel.name());
System.assertEquals('ERROR', lastLevel.name());
System.debug(LoggingLevel.ERROR, LoggingLevel.WARN);
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Debug; len(got) != 3 || got[0] != "system qualified level" || got[1] != "qualified variable level" || got[2] != "WARN" {
		t.Fatalf("debug lines = %#v, want %#v", got, []string{"system qualified level", "qualified variable level", "WARN"})
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

func TestExecGeneratedPlatformEnumValuesForSurfacePacket(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert(AccessType.values().size() >= 4);
System.assert(CallbackStatus.values().size() > 0);
System.assert(CustomizationType.values().size() > 0);
System.assert(DomainType.values().size() > 0);
System.assert(JSONToken.values().size() > 0);
System.assert(LoggingLevel.values().size() >= 8);
System.assert(OrgMetricPublishTypeEnum.values().size() > 0);
System.assert(OrgMetricServiceEnum.values().size() > 0);
System.assert(OrgMetricTypeEnum.values().size() > 0);
System.assert(ParentJobResult.values().size() > 0);
System.assert(Quiddity.values().size() > 0);
System.assert(RoundingMode.values().size() >= 8);
System.assert(SetupScope.values().size() > 0);
System.assert(StatusCode.values().size() > 0);
System.assert(TriggerOperation.values().size() >= 7);
System.assert(XmlTag.values().size() > 0);
System.assertEquals(StatusCode.FIELD_CUSTOM_VALIDATION_EXCEPTION, StatusCode.valueOf('FIELD_CUSTOM_VALIDATION_EXCEPTION'));
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
System.assertEquals(null, Type.forName('System.String'));
Type systemStringType = Type.forName('System', 'String');
System.assertEquals(null, systemStringType);
System.assertEquals(null, Type.forName('System', 'DefinitelyMissing'));
Type systemDmlType = Type.forName('System', 'DmlException');
System.assertNotEquals(null, systemDmlType);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestTypeForNameRejectsSystemNamespaceAndNonStringArguments(t *testing.T) {
	machine := New(nil)
	result := &Result{}
	value, err := machine.call("Type.forName", []Value{platformScalar("String", "Account")}, nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueObject || value.Type != "Type" || typeValueName(value) != "Account" {
		t.Fatalf("Type.forName platform string = %#v", value)
	}
	value, err = machine.call("Type.forName", []Value{platformScalar("String", "System"), platformScalar("String", "String")}, nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("Type.forName invalid platform namespace string = %#v, want null", value)
	}
	if _, err := machine.call("Type.forName", []Value{Int(1)}, nil, result); err == nil || !strings.Contains(err.Error(), "Type.forName expects String") {
		t.Fatalf("Type.forName integer error = %v", err)
	}
}

func TestExecTypeForNameNamespacedMissingClassReturnsNull(t *testing.T) {
	program, err := CompileAnonymous(`
Type missing = Type.forName('samplepkg', 'ThisClassDoesnotExistInYourOrg');
System.assertEquals(null, missing);
Type existing = Type.forName('samplepkg', 'Present');
System.assertEquals('samplepkg.Present', existing.getName());
Object built = existing.newInstance();
System.assertNotEquals(null, built);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Present", Namespace: "samplepkg", Access: "global"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
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

func TestExecStringValueOfDecimalPreservesLiteralScale(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('0.0', String.valueOf(0.0));
System.assertEquals('12.340', String.valueOf(Decimal.valueOf('12.340')));
Decimal coerced = 0;
System.assertEquals('0', String.valueOf(coerced));
Widget__c widget = new Widget__c();
widget.Score__c = 0;
System.assertEquals('0.0', String.valueOf(widget.Score__c));
Map<String,Object> roundTrip = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(new Map<String,Object>{'score' => widget.Score__c}));
System.assertEquals('0.0', String.valueOf(roundTrip.get('score')));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__c", Fields: map[string]storage.Field{"Score__c": {APIName: "Score__c", Type: storage.FieldDecimal}}}, Records: map[storage.ID]storage.Record{}}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringReplaceStringifiesIDArguments(t *testing.T) {
	program, err := CompileAnonymous(`
Id recordId = '001000000000001';
System.assertEquals('Assigned 001000000000001AAA', 'Assigned {!Id}'.replace('{!Id}', recordId));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringValueOfDateUsesDateText(t *testing.T) {
	program, err := CompileAnonymous(`
String value = String.valueOf(Date.newInstance(2026, 5, 19));
System.assertEquals('2026-05-19', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringValueOfObjectDateUsesMidnightText(t *testing.T) {
	program, err := CompileAnonymous(`
Object value = Date.newInstance(2026, 5, 19);
System.assertEquals('2026-05-19 00:00:00', String.valueOf(value));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringRepeatNegativeCountReturnsEmptyString(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('', 'x'.repeat(-1));
System.assertEquals('', 'x'.repeat('-', -1));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecStubCoreUnsupportedMethodWave(t *testing.T) {
	program, err := CompileAnonymous(`
Date leap = Date.newInstance(2024, 2, 29);
System.assertEquals(60, leap.dayOfYear());
System.assert(Date.isLeapYear(2024));
System.assertEquals('2/25/2024', leap.toStartOfWeek().format());

Datetime stamp = '2024-02-29T23:59:58.250Z';
System.assertEquals(60, stamp.dayOfYearGmt());
System.assertEquals(250, stamp.millisecondGmt());
System.assert(stamp.isSameDay(Datetime.valueOfGmt('2024-02-29T00:00:01Z')));
System.assertEquals(false, stamp.isSameDay(Datetime.valueOfGmt('2024-03-01T00:00:00Z')));
System.assert(stamp.formatLong().contains('2024'));

String gmt = String.valueOfGmt(stamp);
System.assert(gmt.startsWith('2024-02-29 23:59:58'));

Blob pdf = Blob.valueOf('glade').toPdf('stub');
System.assert(pdf.toString().startsWith('%PDF-1.4'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
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

func TestExecSObjectTypeDescribeInsideCustomExceptionConstructor(t *testing.T) {
	ctorProgram, err := CompileAnonymous(`
new AppException(String.format('Unsupported {0}', new List<String>{unsupportedObjectType.getDescribe().getName()}));
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
    throw new AppException(Account.SObjectType);
} catch (AppException e) {
    System.assertEquals('AppException', e.getTypeName());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "AppException",
		SuperClass: "Exception",
		Constructors: []Method{{
			Name:          "AppException.<init>",
			ClassName:     "AppException",
			Params:        []Param{{Name: "unsupportedObjectType", Type: "Schema.SObjectType"}},
			Program:       ctorProgram,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectTypeDescribeParameterInsideConstructor(t *testing.T) {
	ctorProgram, err := CompileAnonymous(`
String message = String.format('Unsupported {0}', new List<String>{unsupportedObjectType.getDescribe().getName()});
System.assertEquals('Unsupported Account', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`new AppException(Account.SObjectType);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "AppException",
		SuperClass: "Exception",
		Constructors: []Method{{
			Name:          "AppException.<init>",
			ClassName:     "AppException",
			Params:        []Param{{Name: "unsupportedObjectType", Type: "Schema.SObjectType"}},
			Program:       ctorProgram,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
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
System.assertEquals('https://local.glade.example/apex/current', defaultURL.toExternalForm());
Test.setCurrentPage(new PageReference('/apex/current'));
PageReference current = ApexPages.currentPage();
current.getParameters().put('mode', 'local');
URL withParams = URL.getCurrentRequestUrl();
System.assertEquals('https://local.glade.example/apex/current?mode=local', withParams.toExternalForm());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeForNameReturnsNullForMissingNamespacedClass(t *testing.T) {
	program, err := CompileAnonymous(`
Type packaged = Type.forName('pkg', 'Thing');
System.assertEquals(null, packaged);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeForNameRejectsQualifiedSystemBuiltinFallback(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(null, Type.forName('System.AssertException'));
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
Decimal scaledNegative = Decimal.valueOf('-0.20');
Double x = Double.valueOf('2.25');
Double signedDouble = Double.valueOf(' +6.25 ');
Decimal bigLong = Decimal.valueOf('3000000000');
Boolean truthy = Boolean.valueOf(' TRUE ');
Boolean falsy = boolean.valueOf('no');
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
System.assertEquals('42', i.format());
System.assertEquals('1,234,567', 1234567.format());
System.assertEquals('9,001', l.format());
System.assertEquals('10,000.5', Decimal.valueOf('10000.5').format());
System.assertEquals('999', Decimal.valueOf('999.0').format());
System.assertEquals('1', Decimal.valueOf('1.00').format());
System.assertEquals('10,000.5', Decimal.valueOf('10000.50').format());
System.assertEquals('2.25', x.format());
System.assertEquals('', 'abc'.left(-1));
System.assertEquals('', 'abc'.right(-1));
System.assertEquals('10.00', Decimal.valueOf('10').setScale(2).toPlainString());
System.assertEquals(12, d.intValue());
System.assertEquals(12, d.longValue());
System.assertEquals(3000000000, bigLong.longValue());
System.assertEquals(12.5, d.doubleValue());
System.assertEquals(12.5, d.abs());
System.assertEquals(2, scaledNegative.abs().scale());
System.assertEquals('0.20', Math.abs(scaledNegative).toPlainString());
System.assertEquals(2, Math.abs(scaledNegative).scale());
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
System.assertEquals(1, Math.signum(12.5));
System.assertEquals(-1, Math.signum(-4));
System.assertEquals(0, Math.signum(0));
System.assertEquals(1, Math.mod(10, 3));
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
System.assert(Math.abs(Math.sin(1.5707963267948966) - 1) < 0.000000000001);
System.assert(Math.abs(Math.cos(0) - 1) < 0.000000000001);
System.assert(Math.abs(Math.tan(0)) < 0.000000000001);
System.assert(Math.abs(Math.cosh(0) - 1) < 0.000000000001);
System.assert(Math.abs(Math.sinh(0)) < 0.000000000001);
System.assert(Math.abs(Math.tanh(0)) < 0.000000000001);
System.assert(Math.abs(Math.acos(1)) < 0.000000000001);
System.assert(Math.abs(Math.asin(1) - 1.5707963267948966) < 0.000000000001);
System.assert(Math.abs(Math.atan(1) - 0.7853981633974483) < 0.000000000001);
System.assert(Math.abs(Math.atan2(1, 1) - 0.7853981633974483) < 0.000000000001);
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
UUID parsedUuid = UUID.fromString(uuid1);
System.assertEquals(uuid1, parsedUuid.toString());
System.assert(parsedUuid.equals(UUID.fromString(uuid1)));
System.assertEquals(parsedUuid.hashCode(), UUID.fromString(uuid1).hashCode());
Version version = new Version(1, 2, 3);
System.assertEquals(1, version.major());
System.assertEquals(2, version.minor());
System.assertEquals(3, version.patch());
System.assertEquals('1.2', new Version(1, 2).toString());
System.assertEquals(0, new Version(1, 95).compareTo(new Version(1, 95, 16)));
System.assertEquals(0, new Version(1, 95, 16).compareTo(new Version(1, 95)));
System.assert(new Version(1, 24).compareTo(new Version(1, 95, 16)) < 0);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecIntegerValueOfReturnsNullForTypedNumericNulls(t *testing.T) {
	program, err := CompileAnonymous(`
Integer integerNull = null;
Decimal decimalNull = null;
Object objectNull = null;
System.assertEquals(null, Integer.valueOf(integerNull));
System.assertEquals(null, Integer.valueOf(decimalNull));
System.assertEquals(null, Integer.valueOf(objectNull));
Map<String,Object> values = new Map<String,Object>();
System.assertEquals(null, Integer.valueOf(values.get('missing')));
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
String dateValueOfObjectError = '';
try {
	Date.ValueOf((Object)'2026-05-07');
} catch (TypeException e) {
	dateValueOfObjectError = e.getMessage();
}
System.assertEquals('Invalid date: 2026-05-07', dateValueOfObjectError);
System.assertEquals('INFO', logginglevel.info.name());
System.assertEquals('ERROR', logginglevel.ERROR.name());
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
	root := t.TempDir()
	writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeVMTestFile(t, filepath.Join(root, "force-app/main/default/staticresources/resetcss.resource"), "body")
	writeVMTestFile(t, filepath.Join(root, "force-app/main/default/staticresources/resetcss.resource-meta.xml"), `<StaticResource><contentType>text/css</contentType><cacheControl>Public</cacheControl></StaticResource>`)
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

func writeVMTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericStdlibRejectsInvalidInputs(t *testing.T) {
	tests := []string{
		"Integer.valueOf('not an integer');",
		"String s = null;\nInteger.valueOf(s);",
		"Integer.valueOf('  ');",
		"Integer.valueOf('42.0');",
		"Integer.valueOf('2147483648');",
		"Long.valueOf('9x');",
		"String s = null;\nLong.valueOf(s);",
		"Long.valueOf('9223372036854775808');",
		"String s = null;\nBoolean.valueOf(s);",
		"Decimal.valueOf('NaN');",
		"String s = null;\nDecimal.valueOf(s);",
		"String s = null;\nDouble.valueOf(s);",
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

func TestExecDecimalScaleSupportsLargePositiveScale(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal d = Decimal.valueOf('1.234567890123456789');
System.assertEquals(1.234567890123456789, d.setScale(18, RoundingMode.HALF_UP));
System.assertEquals(1.234567890123456789000000000000000, d.setScale(33, RoundingMode.HALF_UP));
Boolean highCaught = false;
try {
	d.setScale(34, RoundingMode.HALF_UP);
} catch (MathException e) {
	highCaught = true;
	System.assertEquals('Invalid scale: 34', e.getMessage());
}
System.assert(highCaught);
Boolean lowCaught = false;
try {
	d.setScale(-34, RoundingMode.HALF_UP);
} catch (MathException e) {
	lowCaught = true;
	System.assertEquals('Invalid scale: -34', e.getMessage());
}
System.assert(lowCaught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalScaleUsesLocalDecimalStringTies(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal nickel = Decimal.valueOf('1.005');
Decimal bankersDown = Decimal.valueOf('2.685');
Decimal bankersUp = Decimal.valueOf('2.675');
Decimal negative = Decimal.valueOf('-1.005');
Decimal negativeDirected = Decimal.valueOf('-1.25');
System.assertEquals(1.01, nickel.setScale(2));
System.assertEquals(1.00, nickel.setScale(2, RoundingMode.valueOf('HALF_DOWN')));
System.assertEquals(2.68, bankersDown.setScale(2, RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(2.68, bankersUp.setScale(2, RoundingMode.valueOf('HALF_EVEN')));
System.assertEquals(-1.01, negative.setScale(2, RoundingMode.valueOf('HALF_UP')));
System.assertEquals(-1.2, negativeDirected.setScale(1, RoundingMode.CEILING));
System.assertEquals(-1.3, negativeDirected.setScale(1, RoundingMode.FLOOR));
System.assertEquals(130, Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP));
System.assertEquals(120, Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_DOWN));
System.assertEquals(2, Decimal.valueOf('2.5').round());
System.assertEquals(-2, Decimal.valueOf('-2.5').round());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalEncodingCryptoOracleEdges(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(2, Decimal.valueOf('2.5').round());
System.assertEquals(-2, Decimal.valueOf('-2.5').round());
System.assertEquals(3, Decimal.valueOf('2.5').round(RoundingMode.HALF_UP));
System.assertEquals(12, Decimal.valueOf('12.5').round(RoundingMode.HALF_EVEN));
System.assertEquals(14, Decimal.valueOf('13.5').round(RoundingMode.HALF_EVEN));
System.assertEquals(1.234567890123456789, Decimal.valueOf('1.234567890123456789').setScale(18, RoundingMode.HALF_UP));
System.assertEquals(1.3E+2, Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP));
System.assertEquals(1.20, Decimal.valueOf('1.20').setScale(2, RoundingMode.UNNECESSARY));
try {
	Decimal.valueOf('1.21').setScale(1, RoundingMode.UNNECESSARY);
	System.assert(false, 'expected MathException');
} catch (MathException e) {
	System.assertEquals('Scale insufficient', e.getMessage());
}

System.assertEquals('A+B%2B', EncodingUtil.urlEncode('A B+', 'UTF-8'));
System.assertEquals('A B+Ω', EncodingUtil.urlDecode('A+B%2B%CE%A9', 'utf8'));
System.assertEquals('caf%E9+trail', EncodingUtil.urlEncode(EncodingUtil.urlDecode('caf%E9+trail', 'ISO-8859-1'), 'ISO-8859-1'));
System.assertEquals('café trail', EncodingUtil.urlDecode('caf%E9+trail', 'latin1'));
System.assertEquals('caf%3F', EncodingUtil.urlEncode(EncodingUtil.urlDecode('caf%E9', 'ISO-8859-1'), 'US-ASCII'));
System.assertEquals('%3F', EncodingUtil.urlEncode('Ω', 'ISO-8859-1'));
System.assertEquals('�', EncodingUtil.urlDecode('%E9', 'ASCII'));
System.assertEquals('x', EncodingUtil.urlEncode('x', 'UTF-16'));
System.assertEquals('%FE%FF%03%A9', EncodingUtil.urlEncode('Ω', 'UTF-16'));
System.assertEquals('x', EncodingUtil.urlDecode('x', 'UTF-16'));
System.assertEquals('Ω', EncodingUtil.urlDecode('%FE%FF%03%A9', 'UTF-16'));
System.assertEquals('A B+Ω', EncodingUtil.urlDecode(EncodingUtil.urlEncode('A B+Ω', 'UTF-16'), 'UTF-16'));

System.assertEquals('900150983cd24fb0d6963f7d28e17f72', EncodingUtil.convertToHex(Crypto.generateDigest('MD5', Blob.valueOf('abc'))));
System.assertEquals('a9993e364706816aba3e25717850c26c9cd0d89d', EncodingUtil.convertToHex(Crypto.generateDigest('SHA1', Blob.valueOf('abc'))));
System.assertEquals(EncodingUtil.convertToHex(Crypto.generateDigest('SHA1', Blob.valueOf('abc'))), EncodingUtil.convertToHex(Crypto.generateDigest('SHA-1', Blob.valueOf('abc'))));
System.assertEquals('ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad', EncodingUtil.convertToHex(Crypto.generateDigest('SHA256', Blob.valueOf('abc'))));
System.assertEquals(EncodingUtil.convertToHex(Crypto.generateDigest('SHA256', Blob.valueOf('abc'))), EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf('abc'))));
System.assertEquals('3a985da74fe225b2045c172d6bd390bd855f086e3e9d525b46bfe24511431532', EncodingUtil.convertToHex(Crypto.generateDigest('SHA3-256', Blob.valueOf('abc'))));
try {
	Crypto.generateDigest('SHA-999', Blob.valueOf('abc'));
	System.assert(false, 'expected SecurityException');
} catch (SecurityException e) {
	System.assertEquals('SHA-999 MessageDigest not available', e.getMessage());
}
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
System.assert(!names.contains('A'));
System.assert(!names.containsAll(new List<String>{'A'}));
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
Map<String,Integer> counts = new Map<String,Integer>();
counts.put('b', 2);
counts.put('a', 1);
Map<String,Integer> copiedCounts = new Map<String,Integer>(counts);
System.assertEquals(counts, copiedCounts);
System.assertEquals('Map{a=1, b=2}', copiedCounts.toString());
List<Integer> orderedValues = copiedCounts.values();
System.assertEquals(2, orderedValues.get(0));
System.assertEquals(1, orderedValues.get(1));
Map<String,Integer> clonedCounts = copiedCounts.clone();
clonedCounts.put('a', 9);
System.assertEquals(1, copiedCounts.get('a'));
System.assertEquals(9, clonedCounts.get('a'));
List<Integer> clonedOrderedValues = clonedCounts.values();
System.assertEquals(2, clonedOrderedValues.get(0));
System.assertEquals(9, clonedOrderedValues.get(1));
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

func TestExecMapValuesOrderSurvivesTypedAssignmentAndIndexProjection(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Integer> source = new Map<String, Integer>();
source.put('0', 10);
source.put('1', 20);
source.put('2', 30);
Map<String, Integer> assigned = source;
Map<Integer, Integer> byIndex = new Map<Integer, Integer>();
for (Integer i = 0; i < assigned.size(); i++) {
    byIndex.put(i, assigned.values()[i]);
}
System.assertEquals(10, byIndex.get(0));
System.assertEquals(20, byIndex.get(1));
System.assertEquals(30, byIndex.get(2));
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

func TestExecListSortPreservesOrderForInconsistentNegativeComparable(t *testing.T) {
	compareProgram, err := CompileAnonymous("return -1;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box first = new Box();
first.Name = 'first';
Box second = new Box();
second.Name = 'second';
List<Box> boxes = new List<Box>{first, second};
boxes.sort();
System.assertEquals('first', boxes.get(0).Name);
System.assertEquals('second', boxes.get(1).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Box",
		Interfaces: []string{"Comparable"},
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
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

func TestExecBoxedObjectStringInequalityUsesStringValue(t *testing.T) {
	program, err := CompileAnonymous(`
Object left = 'old';
Object right = 'new';
System.assert(left != right);
System.assertEquals(false, left == right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSortObjectKeysKeepsPrimitiveValuesComparable(t *testing.T) {
	program, err := CompileAnonymous(`
Map<Object, String> labels = new Map<Object, String>();
labels.put(Decimal.valueOf('20.00'), 'twenty');
labels.put(Decimal.valueOf('10.00'), 'ten');

List<Object> keys = new List<Object>();
keys.addAll(labels.keySet());
keys.sort();

System.assertEquals(Decimal.valueOf('10.00'), keys.get(0));
System.assertEquals(Decimal.valueOf('20.00'), keys.get(1));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecListSortObjectKeysAllowsNullWithPrimitiveValues(t *testing.T) {
	program, err := CompileAnonymous(`
Map<Object, String> labels = new Map<Object, String>();
labels.put(Decimal.valueOf('20.00'), 'twenty');
labels.put(null, 'none');
labels.put(Decimal.valueOf('10.00'), 'ten');

List<Object> keys = new List<Object>();
keys.addAll(labels.keySet());
keys.sort();

System.assertEquals(null, keys.get(0));
System.assertEquals(Decimal.valueOf('10.00'), keys.get(1));
System.assertEquals(Decimal.valueOf('20.00'), keys.get(2));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
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

`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibSetDeepClonePreservesSObjectIsolationAndMembership(t *testing.T) {
	program, err := CompileAnonymous(`
Account acme = new Account(Id = '001B000001DVM9tIAH', Name = 'Acme');
Set<Account> original = new Set<Account>{acme};
Set<Account> cloned = original.deepClone();
Account clonedAcme = cloned.iterator().next();
clonedAcme.Name = 'Clone';
Account originalAcme = original.iterator().next();
System.assertEquals('Acme', originalAcme.Name);
System.assertEquals('Clone', cloned.iterator().next().Name);

Set<String> names = new Set<String>{'a', 'b'};
Set<String> clonedNames = names.deepClone();
clonedNames.add('c');
System.assertEquals(false, names.contains('c'));
System.assertEquals(true, clonedNames.contains('c'));
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

func TestCollectionStdlibCloneValuePreservesApexMocksProviderCycles(t *testing.T) {
	provider := Object("framework_ApexMocks")
	recorder := Object("framework_MethodReturnValueRecorder")
	proxy := Object("ISchemaService")
	proxy.Fields["__gladeStubProvider"] = provider
	recorder.Fields["proxy"] = proxy
	provider.Fields["methodReturnValueRecorder"] = recorder

	cloned := cloneValue(provider)
	if cloned.Fields["methodReturnValueRecorder"].Fields["proxy"].Fields["__gladeStubProvider"].Fields["methodReturnValueRecorder"].Kind == ValueNull {
		t.Fatalf("cloned ApexMocks provider cycle lost recorder field")
	}
	if cloned.Fields["methodReturnValueRecorder"].Fields["proxy"].Fields["__gladeStubProvider"].Fields["methodReturnValueRecorder"].Type != "framework_MethodReturnValueRecorder" {
		t.Fatalf("cloned ApexMocks provider cycle did not preserve recorder object")
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
System.assertEquals('nil', labels.get(null));
Set<String> nullKeys = labels.keySet();
System.assert(nullKeys.contains(null));
System.assertEquals(null, labels.remove('blank'));
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
try {
    new Map<Id, Account>(maybeAccounts);
    System.assert(false, 'Expected null list map constructor to throw');
} catch (System.NullPointerException e) {
    System.assert(e.getMessage().contains('Attempt to de-reference a null object'));
}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapFromCastedSObjectListTrustsDeclaredValueType(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> raw = new List<SObject>{new Account(Id = '001000000000001AAA', Name = 'Acme')};
List<Contact> contacts = (List<Contact>) raw;
Map<Id, Contact> byId = new Map<Id, Contact>(contacts);
System.assertEquals(1, byId.size());
System.assert(byId.containsKey('001000000000001AAA'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGenericSObjectMapFromListDoesNotExposeConcreteSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records = new List<SObject>{new Account(Id = '001000000000001AAA', Name = 'Acme')};
Map<Id, SObject> byId = new Map<Id, SObject>(records);
try {
    byId.getSObjectType();
    System.assert(false, 'expected TypeException');
} catch (System.TypeException e) {
    System.assert(e.getMessage().contains('concrete SObject value type'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecEmptyNonSObjectListCannotCastToSObjectList(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> values = new List<String>();
try {
    List<Account> accounts = (List<Account>)values;
    System.assert(false, 'expected TypeException');
} catch (System.TypeException e) {
    System.assert(e.getMessage().contains('List<String>'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibUnsavedSObjectMapKeysUseFieldValues(t *testing.T) {
	program, err := CompileAnonymous(`
Account first = new Account(Name = 'Acme');
Account second = new Account(Name = 'Beta');
Map<Account, String> labels = new Map<Account, String>();
labels.put(first, 'first');
labels.put(second, 'second');
System.assertEquals(2, labels.size());
System.assertEquals('first', labels.get(first));
System.assertEquals('second', labels.get(second));
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
			name: "null SObject row",
			body: "List<Account> accounts = new List<Account>{null}; Map<Id, Account> byId = new Map<Id, Account>(accounts);",
			want: "requires non-null SObject at index 0",
		},
		{
			name: "wrong SObject value type",
			body: "List<Account> accounts = new List<Account>{new Account(Id = '001B000001DVM9tIAH')}; Map<Id, Contact> byId = new Map<Id, Contact>(accounts);",
			want: "value at index 0: cannot assign Account to Contact",
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

func TestExecCollectionStdlibSObjectStringMapUsesIdKey(t *testing.T) {
	program, err := CompileAnonymous(`
Account first = new Account(Id = '001B000001DVM9tIAH', Name = 'Acme');
Account second = new Account(Id = '001B000001DVM9uIAH', Name = 'Beta');
Map<String, Account> byIdText = new Map<String, Account>(new List<Account>{first, second});
System.assertEquals(2, byIdText.size());
System.assertEquals(first.Id, byIdText.get(String.valueOf(first.Id)).Id);
System.assertEquals(second.Id, byIdText.get(String.valueOf(second.Id)).Id);
		`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibSObjectMapDuplicateIdsUseLastValue(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Id = '001B000001DVM9tIAH', Name = 'first');
Account b = new Account(Id = '001B000001DVM9tIAH', Name = 'second');
Map<Id, Account> byId = new Map<Id, Account>(new List<Account>{a, b});
System.assertEquals(1, byId.size());
System.assertEquals('second', byId.get(a.Id).Name);
byId.putAll(new List<Account>{a});
System.assertEquals(1, byId.size());
System.assertEquals('first', byId.get(a.Id).Name);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecCollectionStdlibSObjectMapPreservesListOrderForValues(t *testing.T) {
	program, err := CompileAnonymous(`
Account b = new Account(Id = '001B000001DVM9tIAH', Name = 'second');
Account a = new Account(Id = '001B000001DVM9sIAH', Name = 'first');
Map<Id, Account> byId = new Map<Id, Account>(new List<Account>{b, a});
List<Account> values = byId.values();
System.assertEquals('second', values[0].Name);
System.assertEquals('first', values[1].Name);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSetRemoveAllMatchesSObjectFieldTokensByValue(t *testing.T) {
	program, err := CompileAnonymous(`
Set<Schema.SObjectField> fields = new Set<Schema.SObjectField>{User.ContactId, User.Email};
fields.removeAll(new Set<Schema.SObjectField>{User.ContactId});
System.assert(!fields.contains(User.ContactId));
System.assert(fields.contains(User.Email));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalNegativeScalePreservesObservableText(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal d = Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP);
System.assertEquals(130, d);
System.assertEquals(-1, d.scale());
System.assertEquals(2, d.precision());
System.assertEquals('1.3E+2', String.valueOf(d));
System.assertEquals('130', d.toPlainString());

Decimal d2 = Decimal.valueOf('125').setScale(-2, RoundingMode.HALF_UP);
System.assertEquals(100, d2);
System.assertEquals(-2, d2.scale());
System.assertEquals(1, d2.precision());
System.assertEquals('1E+2', String.valueOf(d2));
System.assertEquals('100', d2.toPlainString());

Decimal neg = Decimal.valueOf('-125').setScale(-1, RoundingMode.HALF_UP);
System.assertEquals(-130, neg);
System.assertEquals(-1, neg.scale());
System.assertEquals(2, neg.precision());
System.assertEquals('-1.3E+2', String.valueOf(neg));
System.assertEquals('-130', neg.toPlainString());

	Decimal chained = Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP).setScale(1, RoundingMode.HALF_UP);
System.assertEquals(130.0, chained);
System.assertEquals(-1, Decimal.valueOf('125').setScale(-1, RoundingMode.HALF_UP).stripTrailingZeros().scale());

Decimal zero = Decimal.valueOf('0').setScale(-1);
System.assertEquals(0, zero);
System.assertEquals(-1, zero.scale());
System.assertEquals(1, zero.precision());
System.assertEquals('0E+1', String.valueOf(zero));
System.assertEquals('0', zero.toPlainString());
System.assertEquals(0, zero.stripTrailingZeros().scale());
System.assertEquals('0', String.valueOf(zero.stripTrailingZeros()));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemComparableSortContract(t *testing.T) {
	compareProgram, err := CompileAnonymous(`
Sorter otherSorter = (Sorter) other;
if (this.Score == otherSorter.Score) {
	return 0;
}
if (this.Score > otherSorter.Score) {
	return 1;
}
return -1;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Sorter a = new Sorter();
a.Score = 3;
a.Label = 'third';
Sorter b = new Sorter();
b.Score = 1;
b.Label = 'first';
Sorter c = new Sorter();
c.Score = 2;
c.Label = 'second';
List<Sorter> items = new List<Sorter>{a, b, c};
items.sort();
System.assertEquals('first', items.get(0).Label);
System.assertEquals('second', items.get(1).Label);
System.assertEquals('third', items.get(2).Label);

items.sort();
System.assertEquals('first', items.get(0).Label);
System.assertEquals('second', items.get(1).Label);
System.assertEquals('third', items.get(2).Label);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Sorter",
		Interfaces: []string{"Comparable"},
		Fields: map[string]Field{
			"Score": {Name: "Score", Type: "Integer"},
			"Label": {Name: "Label", Type: "String"},
		},
		Methods: map[string]Method{
			"compareTo": {
				Name:       "Sorter.compareTo",
				ClassName:  "Sorter",
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

func TestExecSystemComparableSortWithNullValues(t *testing.T) {
	compareProgram, err := CompileAnonymous(`
if (other == null) {
	return 0;
}
Sorter otherSorter = (Sorter) other;
if (this.Score == otherSorter.Score) {
	return 0;
}
if (this.Score > otherSorter.Score) {
	return 1;
}
return -1;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Sorter a = new Sorter();
a.Score = 2;
a.Label = 'second';
Sorter b = new Sorter();
b.Score = 1;
b.Label = 'first';
List<Sorter> items = new List<Sorter>{null, a, null, b, null};
items.sort();
System.assertEquals(null, items.get(0));
System.assertEquals(null, items.get(1));
System.assertEquals(null, items.get(2));
System.assertEquals('first', items.get(3).Label);
System.assertEquals('second', items.get(4).Label);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Sorter",
		Interfaces: []string{"Comparable"},
		Fields: map[string]Field{
			"Score": {Name: "Score", Type: "Integer"},
			"Label": {Name: "Label", Type: "String"},
		},
		Methods: map[string]Method{
			"compareTo": {
				Name:       "Sorter.compareTo",
				ClassName:  "Sorter",
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

func TestExecSystemComparableTypeQuery(t *testing.T) {
	compareProgram, err := CompileAnonymous(`
Sorter otherSorter = (Sorter) other;
if (this.Score == otherSorter.Score) {
	return 0;
}
if (this.Score > otherSorter.Score) {
	return 1;
}
return -1;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Sorter s = new Sorter();
s.Score = 5;
Object obj = s;
System.assert(obj instanceof Comparable);
System.assert(obj instanceof System.Comparable);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Sorter",
		Interfaces: []string{"Comparable"},
		Fields: map[string]Field{
			"Score": {Name: "Score", Type: "Integer"},
		},
		Methods: map[string]Method{
			"compareTo": {
				Name:       "Sorter.compareTo",
				ClassName:  "Sorter",
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
