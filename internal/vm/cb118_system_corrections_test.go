package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCB118AsyncInfoUnsupportedContextMessage(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
	System.AsyncInfo.getCurrentQueueableStackDepth();
} catch (AsyncException e) {
	message = e.getMessage();
}
System.assertEquals('getCurrentQueueableStackDepth is not allowed outside a Queueable of Finalizer execution', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118JSONGeneratorGetAsStringOpenContainerReturnsValue(t *testing.T) {
	program, err := CompileAnonymous(`
JSONGenerator generator = JSON.createGenerator(false);
generator.writeStartObject();
Boolean completed = false;
try {
	generator.getAsString();
	completed = true;
} catch (Exception e) {
}
System.assertEquals(true, completed);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118DateValueOfObjectUsesObjectParseBehavior(t *testing.T) {
	program, err := CompileAnonymous(`
Object dateObject = '2024-02-29';
String message = '';
try {
	Date.valueOf(dateObject);
} catch (TypeException e) {
	message = e.getMessage();
}
System.assertEquals('Invalid date: 2024-02-29', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118PageReferenceMissingResourceException(t *testing.T) {
	program, err := CompileAnonymous(`
String typeName = '';
String message = '';
try {
	PageReference.forResource('Images');
} catch (Exception e) {
	typeName = e.getTypeName();
	message = e.getMessage();
}
System.assertEquals('System.InvalidParameterValueException', typeName);
System.assertEquals('Static Resource does not exist', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118XmlTagValuesPreserveDeclarationOrder(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('["START_ELEMENT","END_ELEMENT","PROCESSING_INSTRUCTION","CHARACTERS","COMMENT","SPACE","START_DOCUMENT","END_DOCUMENT","ENTITY_REFERENCE","ATTRIBUTE","DTD","CDATA","NAMESPACE","NOTATION_DECLARATION","ENTITY_DECLARATION"]', JSON.serialize(XmlTag.values()));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118MathJSONPreservesDoubleIntegralFormatting(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('[5,3,2,7,3,8.0,2,3.0,0.0,3.141592653589793,2.718281828459045]', JSON.serialize(new List<Object>{Math.abs(-5), Math.ceil(2.1), Math.floor(2.9), Math.max(3, 7), Math.min(3, 7), Math.pow(2, 3), Math.round(2.5), Math.sqrt(9), Math.sin(0.0), Math.PI, Math.E}));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118LocationJSONPreservesDoubleIntegralFormatting(t *testing.T) {
	program, err := CompileAnonymous(`
Address address = new Address();
System.assertEquals('[null,null,null,null,null,60.0,-154.0]', JSON.serialize(new List<Object>{address.getStreet(), address.getCity(), address.getState(), address.getPostalCode(), address.getCountry(), Location.newInstance(60.0, -154.0).getLatitude(), Location.newInstance(60.0, -154.0).getLongitude()}));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118DoubleRoundUsesHalfUp(t *testing.T) {
	program, err := CompileAnonymous(`
Double value = Double.valueOf('2.5');
System.assertEquals(3, value.round());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118CollectionToStringUsesApexForms(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> values = new List<Integer>{3, 1, 2};
Set<String> names = new Set<String>{'a', 'b'};
Map<String, Integer> counts = new Map<String, Integer>{'a' => 1};
System.assertEquals('(3, 1, 2)', String.valueOf(values));
System.assertEquals('{a, b}', names.toString());
System.assertEquals('{a=1}', counts.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118XmlStreamReaderWithoutDeclarationHasNullVersion(t *testing.T) {
	program, err := CompileAnonymous(`
XmlStreamReader reader = new XmlStreamReader('<root/>');
System.assertEquals(null, reader.getVersion());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118QueryExceptionConstructionCapturesLineAndCause(t *testing.T) {
	program, err := CompileAnonymous(`List<Object> rows = new List<Object>();
try {
Exception ex = new QueryException('message');
Exception cause = new QueryException('cause');
ex.initCause(cause);
ex.setMessage('changed');
System.assertEquals('changed', ex.getMessage());
System.assertEquals('System.QueryException', ex.getTypeName());
System.assertEquals('System.QueryException', ex.getCause().getTypeName());
System.assertEquals(3, ex.getLineNumber());
System.assertEquals('AnonymousBlock: line 3, column 1\nCaused by\nAnonymousBlock: line 4, column 1', ex.getStackTraceString());
System.assertEquals(0, ex.getInaccessibleFields().size());
System.assertEquals(true, ex.equals(ex));
System.assertEquals('System.QueryException: changed', ex.toString());
} catch (Exception e) {
throw e;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118SObjectCloneWithoutIdIsNotClone(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
Account cloned = (Account)account.clone(false, false, false, false);
System.assertEquals(false, cloned.isClone());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118HttpResponseHeaderKeysPreserveCasing(t *testing.T) {
	program, err := CompileAnonymous(`
HttpResponse response = new HttpResponse();
response.setHeader('Content-Type', 'text/plain');
System.assertEquals('["Content-Type"]', JSON.serialize(response.getHeaderKeys()));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118AccessLevelStringUsesRecursiveObjectForm(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('AccessLevel:[SYSTEM_MODE=(already output), USER_MODE=AccessLevel:[SYSTEM_MODE=(already output), USER_MODE=(already output), currentAccessPermissions=USER_MODE, permSetId=null], currentAccessPermissions=SYSTEM_MODE, permSetId=null]', String.valueOf(AccessLevel.SYSTEM_MODE));
System.assertEquals('AccessLevel:[SYSTEM_MODE=AccessLevel:[SYSTEM_MODE=(already output), USER_MODE=(already output), currentAccessPermissions=SYSTEM_MODE, permSetId=null], USER_MODE=(already output), currentAccessPermissions=USER_MODE, permSetId=null]', String.valueOf(AccessLevel.USER_MODE));
System.assertEquals('AccessLevel:[SYSTEM_MODE=AccessLevel:[SYSTEM_MODE=(already output), USER_MODE=AccessLevel:[SYSTEM_MODE=(already output), USER_MODE=(already output), currentAccessPermissions=USER_MODE, permSetId=null], currentAccessPermissions=SYSTEM_MODE, permSetId=null], USER_MODE=(already output), currentAccessPermissions=SYSTEM_MODE, permSetId=null]', String.valueOf(AccessLevel.SYSTEM_MODE.clone()));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118JSONUntypedNumberErrorUsesFieldBoundaryLocation(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
JSON.deserializeUntyped('{"whole":12,"big":9223372036854775808,"ratio":1.25}');
} catch (JSONException e) {
message = e.getMessage();
}
System.assertEquals('For input string: "9223372036854775808" at [line:1, column:12]', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB118JSONStrictUnknownSObjectFieldUsesPlatformMessage(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
JSON.deserializeStrict('{"Name":"Acme","NoSuchField__c":"x"}', Account.class);
} catch (JSONException e) {
message = e.getMessage();
}
System.assertEquals('No such column \'NoSuchField__c\' on sobject of type Account', message);
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
