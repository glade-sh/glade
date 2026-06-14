package visualforce

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestEvaluateVisualforceExpressionOperatorsAndGlobals(t *testing.T) {
	machine := vm.New(nil)
	machine.SetCurrentPageURL("/apex/Edit?id=001000000000001")
	ctx := &ExpressionContext{
		VM:               machine,
		Controller:       vm.Object("EditController"),
		Scope:            NewScopeStack(),
		ProjectNamespace: "pkg",
	}
	ctx.Controller.Fields["amount"] = vm.Int(42)
	ctx.Controller.Fields["name"] = vm.String("Acme")

	cases := []struct {
		expr string
		want string
	}{
		{"amount + 8", "50"},
		{"amount - 2", "40"},
		{"amount * 2", "84"},
		{"amount / 2", "21"},
		{"IF(amount > 40, 'big', 'small')", "big"},
		{"IF(amount >= 42, 'yes', 'no')", "yes"},
		{"IF(amount < 50, 'yes', 'no')", "yes"},
		{"IF(amount <= 41, 'yes', 'no')", "no"},
		{"IF(amount == 42, 'yes', 'no')", "yes"},
		{"IF(amount != 41, 'yes', 'no')", "yes"},
		{"IF(TRUE, 'yes', 'no')", "yes"},
		{"IF(FALSE, 'yes', 'no')", "no"},
		{"NOT(ISBLANK(name))", "true"},
		{"AND(amount > 40, name == 'Acme')", "true"},
		{"OR(amount < 40, name == 'Acme')", "true"},
		{"!(amount < 40)", "true"},
		{"(amount + 8) / 2", "25"},
		{"UPPER(name)", "ACME"},
		{"LOWER('Trail Head')", "trail head"},
		{"$CurrentPage.parameters.id", "001000000000001"},
		{"namespace", "pkg"},
	}
	for _, tc := range cases {
		got, err := EvaluateExpression(tc.expr, ctx)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if got != tc.want {
			t.Fatalf("%s = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

func TestRenderExpressionTemplateKeepsExistingTextReplacement(t *testing.T) {
	ctx := &ExpressionContext{Controller: vm.Object("EditController")}
	ctx.Controller.Fields["amount"] = vm.Int(42)

	got, err := RenderExpressionTemplate("total={!amount + 8}; name={!'Acme'}", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "total=50; name=Acme" {
		t.Fatalf("template = %q", got)
	}
}

func TestRenderExpressionTemplateIgnoresBraceInsideStringLiteral(t *testing.T) {
	got, err := RenderExpressionTemplate(`value={!IF(TRUE, '}', 'no')} tail`, &ExpressionContext{})
	if err != nil {
		t.Fatal(err)
	}
	if got != `value=} tail` {
		t.Fatalf("template = %q", got)
	}
}

func TestRenderExpressionTemplateDoesNotReevaluateReplacementText(t *testing.T) {
	ctx := &ExpressionContext{Controller: vm.Object("ProbeController")}
	ctx.Controller.Fields["first"] = vm.String("{!second}")
	ctx.Controller.Fields["second"] = vm.String("expanded")

	got, err := RenderExpressionTemplate("value={!first}; other={!second}", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "value={!second}; other=expanded" {
		t.Fatalf("template = %q", got)
	}
}

func TestEvaluateVisualforceURLFunctions(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"URLENCODE('Trail Head')", "Trail+Head"},
		{"URLENCODE('a/b?x=1&name=Snow café')", "a%2Fb%3Fx%3D1%26name%3DSnow+caf%C3%A9"},
		{"URLDECODE('a%2Fb%3Fx%3D1%26name%3DSnow+caf%C3%A9')", "a/b?x=1&name=Snow café"},
	}
	for _, tc := range cases {
		got, err := EvaluateExpression(tc.expr, &ExpressionContext{})
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if got != tc.want {
			t.Fatalf("%s = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

func TestEvaluateVisualforceEncodeFunctions(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"JSENCODE('quote \" and </script>')", `quote \" and <\/script>`},
		{"HTMLENCODE('<b>Tom & Jerry</b>')", "&lt;b&gt;Tom &amp; Jerry&lt;/b&gt;"},
	}
	for _, tc := range cases {
		got, err := EvaluateExpression(tc.expr, &ExpressionContext{})
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if got != tc.want {
			t.Fatalf("%s = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

func TestEvaluateVisualforceExpressionFunctionsForParity(t *testing.T) {
	ctx := &ExpressionContext{Controller: vm.Object("FormulaController")}
	ctx.Controller.Fields["status"] = vm.String("Cold")
	ctx.Controller.Fields["blank"] = vm.String(" ")
	ctx.Controller.Fields["missing"] = vm.Null

	cases := []struct {
		expr string
		want string
	}{
		{"CASE(status, 'Hot', 'open', 'Cold', 'closed', 'unknown')", "closed"},
		{"CASE(status, 'Hot', 'open', 'unknown')", "unknown"},
		{"BLANKVALUE(blank, 'fallback')", "fallback"},
		{"BLANKVALUE(status, 'fallback')", "Cold"},
		{"NULLVALUE(missing, 'fallback')", "fallback"},
		{"NULLVALUE(blank, 'fallback')", " "},
		{"VALUE('42') + 8", "50"},
		{"VALUE('10.5') + 1", "11.5"},
	}
	for _, tc := range cases {
		got, err := EvaluateExpression(tc.expr, ctx)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if got != tc.want {
			t.Fatalf("%s = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

func TestRenderExpressionTemplateResolvesVisualforceSchemaAndContextGlobals(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName:   "Account",
		Label:     "Account",
		KeyPrefix: "001",
		Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
		},
	}}
	org.Objects["Feature__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Feature__c",
			Metadata: map[string]string{
				"kind":               "customSetting",
				"customSettingsType": "Hierarchy",
			},
			Fields: map[string]storage.Field{
				"Enabled__c": {APIName: "Enabled__c", Type: storage.FieldBoolean},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {
				ID:     "a00000000000001",
				Object: "Feature__c",
				Fields: map[string]storage.Value{
					"Enabled__c": storage.BooleanValue(true),
				},
			},
		},
	}
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"005000000000777AAA": {
			ID:     "005000000000777AAA",
			Object: "User",
			Fields: map[string]storage.Value{
				"Permissions": storage.ListValue(storage.StringValue("ViewSetup")),
			},
		},
	}}
	org.Objects["Site"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"0DM000000000001": {
			ID:     "0DM000000000001",
			Object: "Site",
			Fields: map[string]storage.Value{
				"Name":   storage.StringValue("TrailSite"),
				"Prefix": storage.StringValue("trail"),
			},
		},
	}}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	ctx := &ExpressionContext{VM: machine}

	got, err := RenderExpressionTemplate(
		`Object={!$ObjectType.Account} Field={!$ObjectType.Account.Fields.Name.Label} Permission={!$Permission.ViewSetup} Setting={!$Setup.Feature__c.Enabled__c} Site={!$Site.Name}/{!$Site.Prefix} Component={!$Component.form}`,
		ctx,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "Object=Account Field=Account Name Permission=true Setting=true Site=TrailSite/trail Component=form"
	if got != want {
		t.Fatalf("template = %q, want %q", got, want)
	}
}

func TestRenderExpressionTemplateAppliesURLENCODEInLinkAndOutputText(t *testing.T) {
	ctx := &ExpressionContext{Controller: vm.Object("LinkController")}
	ctx.Controller.Fields["term"] = vm.String("Snow café")

	got, err := RenderExpressionTemplate(`<a href="/apex/Search?q={!URLENCODE(term)}">Search</a><span>{!URLENCODE('Trail Head')}</span>`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := `<a href="/apex/Search?q=Snow+caf%C3%A9">Search</a><span>Trail+Head</span>`
	if got != want {
		t.Fatalf("template = %q, want %q", got, want)
	}
}

func TestResourceExpressionPreservesStaticResourceAndPathCase(t *testing.T) {
	got, err := EvaluateExpression("$Resource.Bundle.css.Site", &ExpressionContext{VM: vm.New(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if got != "/resource/Bundle/css/Site" {
		t.Fatalf("$Resource path = %q", got)
	}
}

func TestRenderExpressionTemplateResolvesVisualforceUserAndOrganizationGlobals(t *testing.T) {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000777EAA"
	org.Objects["Organization"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"00D000000000777EAA": {
			ID:     "00D000000000777EAA",
			Object: "Organization",
			Fields: map[string]storage.Value{
				"Name": storage.StringValue("Trail Local Org"),
			},
		},
	}}
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"005000000000777AAA": {
			ID:     "005000000000777AAA",
			Object: "User",
			Fields: map[string]storage.Value{
				"Username":  storage.StringValue("ada@example.test"),
				"Email":     storage.StringValue("ada-email@example.test"),
				"ProfileId": storage.IDValue("00e000000000777AAA"),
				"UserType":  storage.StringValue("Standard"),
			},
		},
	}}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{
		ID:     "005000000000777AAA",
		Object: "User",
		Fields: map[string]storage.Value{
			"Username":  storage.StringValue("ada@example.test"),
			"Email":     storage.StringValue("ada-email@example.test"),
			"ProfileId": storage.IDValue("00e000000000777AAA"),
		},
	})

	ctx := &ExpressionContext{VM: machine}
	got, err := RenderExpressionTemplate(
		`User: {!$User.Id} {!$User.Username} {!$User.Email} Profile: {!$Profile.Id} Org: {!$Organization.Id} {!$Organization.Name}`,
		ctx,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "User: 005000000000777AAA ada@example.test ada-email@example.test Profile: 00e000000000777AAA Org: 00D000000000777EAA Trail Local Org"
	if got != want {
		t.Fatalf("template = %q, want %q", got, want)
	}
}
