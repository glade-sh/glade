package vm

import (
	"runtime"
	"testing"
)

func TestIPv4ValidationAnonymousAndIsTestRoutes(t *testing.T) {
	t.Logf("GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
String zero = String.join(new List<String>{'0', '0', '0', '0'}, '.');
System.assertEquals('0.0.0.0', zero);
List<String> coldSplit = zero.split('\\.');
System.assertEquals(4, coldSplit.size(), 'cold split');
System.assertEquals(new List<String>{'0', '0', '0', '0'}, coldSplit);
System.assertEquals(coldSplit, Pattern.compile('\\.').split(zero), 'Pattern.split');
String octet = '(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)';
String expression = '^' + octet + '(\\.' + octet + '){3}$';
List<String> addresses = new List<String>{'0.0.0.0', '1.2.3.4', '127.0.0.1', '192.168.1.254', '255.255.255.255'};
for (String address : addresses) {
    System.assert(Pattern.matches(expression, address), 'static regex: ' + address);
    System.assert(Pattern.compile(expression).matcher(address).matches(), 'matcher regex: ' + address);
    List<String> pieces = address.split('\\.');
    System.assertEquals(4, pieces.size(), 'split: ' + address);
    Long numeric = 0L;
    for (String piece : pieces) {
        Integer value = Integer.valueOf(piece);
        System.assertEquals(Long.valueOf(piece), value, 'Integer/Long parsing: ' + piece);
        System.assert(value >= 0 && value <= 255, 'octet: ' + piece);
        System.assertEquals(Decimal.valueOf(piece), Math.floor(Decimal.valueOf(piece)));
        numeric = numeric * 256 + value;
    }
    System.assert(numeric >= 0L && numeric <= 4294967295L, 'packed: ' + address);
}
System.assert(!Pattern.matches(expression, '256.0.0.0'));
System.assert(!Pattern.compile(expression).matcher('0.0.0').matches());
System.assertEquals(0, Integer.valueOf('0'));
System.assertEquals(4294967295L, Long.valueOf('4294967295'));
System.assertEquals(4294967296.0, Math.pow(256.0, 4.0));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if route == "anonymous" {
					_, err = machine.Execute(program)
				} else {
					_, err = machine.ExecuteTestInClass(program, "AddressValidationTest")
				}
				if err != nil {
					t.Fatalf("IPv4 validation: %v", err)
				}
			})
		}
	}
}
