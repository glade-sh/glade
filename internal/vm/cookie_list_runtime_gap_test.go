package vm

import "testing"

func TestExecCookieEquals(t *testing.T) {
	program, err := CompileAnonymous(`
Cookie cookie = new Cookie('sid', 'value', '/', 60, true);
System.assertEquals(true, cookie.equals(cookie));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListCapacityConstructor(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> values = new List<String>(3);
System.assertEquals(3, values.size());
System.assertEquals(null, values[0]);
System.assertEquals(null, values[2]);
values.add('value');
System.assertEquals(4, values.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
