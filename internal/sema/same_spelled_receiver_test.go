package sema

import (
	"strings"
	"testing"
)

// The "Variable does not exist: SObjectType" rule is backed by local rows
// only (M1 S09/S109, B41 H2S2X/H2R1T, R014/C014). A
// parameter or field spelled Contact has no row and keeps the type token for
// Contact.SObjectType.
func TestSameSpelledParameterAndFieldKeepSObjectTypeToken(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public Contact Contact;
  public static Schema.SObjectType fromParameter(Contact Contact) {
    Schema.SObjectType token = Contact.SObjectType;
    return token;
  }
  public Schema.SObjectType fromField() {
    Schema.SObjectType token = Contact.SObjectType;
    return token;
  }
}`,
	}, "67.0")
	if result.HasErrors() {
		t.Fatalf("parameter or field spelled Contact lost the type token: %#v", result.Diagnostics)
	}

	twin := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public Contact Contact;
  public static void fromParameter(Contact Contact) {
    Integer token = Contact.SObjectType;
  }
  public void fromField() {
    Integer token = Contact.SObjectType;
  }
}`,
	}, "67.0")
	tokenErrors := 0
	for _, d := range twin.Diagnostics {
		text := d.Message + " " + d.NativeMessage
		if strings.Contains(text, "Variable does not exist: SObjectType") {
			t.Fatalf("parameter or field read reported the local-only rule: %#v", d)
		}
		if strings.Contains(text, "Schema.SObjectType") {
			tokenErrors++
		}
	}
	if tokenErrors < 2 {
		t.Fatalf("want an Integer mismatch with Schema.SObjectType for the parameter and the field: %#v", twin.Diagnostics)
	}
}

// A parameter spelled like its SObject type in another case is the receiver
// of Account.Name, so the element is the String field value, not the field
// token (native M1 S05: read(Contact contact) { return Contact.Email; } returns
// the parameter's Email at 62.0 and 67.0). Without that binding Account.Name
// stays the token (M1 S14).
func TestSameSpelledParameterIsFieldValueInTokenInitializer(t *testing.T) {
	shadowed := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public static void run(Account account) {
    List<SObjectField> fields = new List<SObjectField>{ Account.Name };
  }
}`,
	}, "67.0")
	if !shadowed.HasErrors() {
		t.Fatal("Account.Name through the account parameter is the String value; the SObjectField initializer must be rejected")
	}
	plain := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public static void run(Account acct) {
    List<SObjectField> fields = new List<SObjectField>{ Account.Name };
  }
}`,
	}, "67.0")
	if plain.HasErrors() {
		t.Fatalf("Account.Name with no account binding is the field token: %#v", plain.Diagnostics)
	}
}

// The IR "Variable does not exist: SObjectType" rejection is the only error
// on the rejected expression (M1 S109 keeps one native error), but errors on
// other expressions of the same line survive: a GLADESEMA018 on another
// initializer and the GLADESEMA019 for a value returned from a void method.
// No capture backs dropping them.
func TestSameSpelledSObjectTypeRejectionKeepsSameLineErrors(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe {
  public static void run() {
    Contact contact = new Contact(LastName = 'x');
    Integer t = Contact.SObjectType; Integer n = 'x'; return 1;
  }
}`,
	}, "67.0")
	rejections, otherAssignments, voidReturns := 0, 0, 0
	for _, d := range result.Diagnostics {
		text := d.Message + " " + d.NativeMessage
		switch {
		case strings.Contains(text, "Variable does not exist: SObjectType"):
			rejections++
		case strings.Contains(text, "Schema.SObjectType"):
			t.Fatalf("second error on the rejected Contact.SObjectType expression: %#v", d)
		case d.Code == "GLADESEMA018":
			otherAssignments++
		case d.Code == "GLADESEMA019":
			voidReturns++
		}
	}
	if rejections != 1 || otherAssignments == 0 || voidReturns == 0 {
		t.Fatalf("want one SObjectType rejection plus the same-line GLADESEMA018 and GLADESEMA019: %#v", result.Diagnostics)
	}
}

// The rejection's range covers the written expression even when whitespace,
// a line break or a comment separates the parts, so the text-path type error
// on the same initializer is still dropped and only native's one error stays
// (M1 S109 shape). Unrelated same-line errors survive.
func TestSameSpelledSObjectTypeRejectionSpansSpacedExpression(t *testing.T) {
	for _, read := range []string{"Contact .SObjectType", "Contact\n    .SObjectType", "Contact /*c*/ .SObjectType"} {
		result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
			"Probe.cls": "public class Probe {\n  public static void run() {\n    Contact contact = new Contact(LastName = 'x');\n    Integer descriptiveTokenName = " + read + "; Integer n = 'x'; return 1;\n  }\n}",
		}, "67.0")
		rejections, otherAssignments, voidReturns := 0, 0, 0
		for _, d := range result.Diagnostics {
			text := d.Message + " " + d.NativeMessage
			switch {
			case strings.Contains(text, "Variable does not exist: SObjectType"):
				rejections++
			case strings.Contains(text, "Schema.SObjectType"):
				t.Fatalf("%q: second error on the rejected expression: %#v", read, d)
			case d.Code == "GLADESEMA018":
				otherAssignments++
			case d.Code == "GLADESEMA019":
				voidReturns++
			}
		}
		if rejections != 1 || otherAssignments == 0 || voidReturns == 0 {
			t.Fatalf("%q: want one SObjectType rejection plus the same-line GLADESEMA018 and GLADESEMA019: %#v", read, result.Diagnostics)
		}
	}
}
