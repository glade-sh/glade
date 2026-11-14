package vm

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

type valueTextCase struct {
	name  string
	value Value
	want  string
}

// valueTextCases covers every value kind and each object formatter that
// Value.String dispatches to, including formatters that omit fields.
func valueTextCases() []valueTextCase {
	big := strings.Repeat("x", 5000)

	record := Object("Account")
	record.Fields["Name"] = String("Acme")
	record.Fields["Id"] = platformScalar("Id", "001000000000001AAA")

	project := Object("Widget")
	project.projectClass = true
	project.Fields["count"] = Int(3)
	project.Fields["label"] = String("w")

	emptyProject := Object("Empty")
	emptyProject.projectClass = true

	plain := Object("Example")
	plain.Fields["name"] = String("sample")

	valueObject := Object("Example")
	valueObject.Fields["value"] = String("short")
	valueObject.Fields["unused"] = String(big)

	page := Object("PageReference")
	page.Fields["url"] = String("/short")
	page.Fields["unused"] = String(big)

	pageNoURL := Object("PageReference")
	pageNoURL.Fields["unused"] = String(big)

	sobjectType := Object("Schema.SObjectType")
	sobjectType.Fields["object"] = String("Account")
	sobjectType.Fields["unused"] = String(big)

	sobjectField := Object("Schema.SObjectField")
	sobjectField.Fields["object"] = String("Account")
	sobjectField.Fields["field"] = String("Name")
	sobjectField.Fields["unused"] = String(big)

	message := Object("Example.Problem")
	message.Fields["message"] = String("went wrong")
	message.Fields["unused"] = String(big)

	locator := Object("Database.QueryLocator")
	locator.Fields["Query"] = String("SELECT Id FROM Account")
	locator.Fields["value"] = String("ignored")

	options := Object("Database.DMLOptions")
	options.Fields["OptAllOrNone"] = Bool(true)
	header := Object("Database.AssignmentRuleHeader")
	header.Fields["UseDefaultRule"] = Bool(true)
	options.Fields["AssignmentRuleHeader"] = header

	convert := Object("Database.LeadConvert")
	convert.Fields["convertedStatus"] = String("Closed")

	action := Object("Invocable.Action")
	action.Fields["name"] = String("act")
	action.Fields["invocations"] = List(Int(1))
	action.Fields["value"] = String("ignored")

	location := newLocation(Int(1), Int(2))

	access := Object("AccessLevel")
	access.Fields["SYSTEM_MODE"] = Bool(true)

	stub := Object("Widget")
	stub.Ref = 7
	stub.Fields["__gladeStubbedType"] = String("Widget")
	stub.Fields["unused"] = String(big)

	request := Object("HttpRequest")
	request.platformHTTPDTO = true
	request.Fields["endpoint"] = String("https://example.test")
	request.Fields["method"] = String("GET")
	request.Fields["unused"] = String(big)

	template := Object("Messaging.RenderEmailTemplateBodyResult")
	template.Fields["errors"] = List()
	template.Fields["mergedBody"] = String("body")
	template.Fields["success"] = Bool(true)

	selfRef := Object("Node")
	selfRef.Ref = 11
	selfRef.Fields["name"] = String("n")
	selfRef.Fields["next"] = selfRef

	numbers := Map()
	numbers.Map["b"] = Int(2)
	numbers.Map["a"] = List(Int(1), Int(2))
	keyed := Map()
	keyed.Map["k"] = String("v")
	keyed.MapKeys["k"] = plain

	queried := Object("Account")
	queried.Fields["Id"] = platformScalar("Id", "001000000000001AAA")
	queried.Fields["Name"] = String("Acme")
	queried.Fields["Industry"] = String("Tech")
	selection := Map()
	for _, name := range []string{"name", "industry", "id"} {
		key := mapKey(String(name))
		selection.Map[key] = Bool(true)
		selection.MapKeys[key] = String(name)
		selection.MapOrder = append(selection.MapOrder, key)
	}
	queried.Fields[sobjectQueriedFieldsField] = selection

	wideObject, wideObjectText := Object("Wide"), "Wide:{"
	wideMap, wideMapText := Map(), "{"
	for i := 0; i < 300; i++ {
		name := fmt.Sprintf("f%03d", i)
		wideObject.Fields[name] = Int(int64(i))
		wideMap.Map[mapKey(String(name))] = Int(int64(i))
		wideMap.MapKeys[mapKey(String(name))] = String(name)
		if i > 0 {
			wideObjectText += ", "
			wideMapText += ", "
		}
		wideObjectText += fmt.Sprintf("%s=%d", name, i)
		wideMapText += fmt.Sprintf("%s=%d", name, i)
	}
	wideObjectText += "}"
	wideMapText += "}"

	return []valueTextCase{
		{"sobject queried order", queried, "Account:{Name=Acme, Industry=Tech, Id=001000000000001AAA}"},
		{"wide object", wideObject, wideObjectText},
		{"wide map", wideMap, wideMapText},
		{"wide map in list", List(wideMap), "List" + "[" + wideMapText + "]"},
		{"null", Null, "null"},
		{"integer", Int(-42), "-42"},
		{"decimal", Value{Kind: ValueDecimal, Text: "1.25"}, "1.25"},
		{"decimal small", Value{Kind: ValueDecimal, Text: "0.000000012300"}, "1.2300E-8"},
		{"decimal small negative", Value{Kind: ValueDecimal, Text: "-0.000000012300"}, "-1.2300E-8"},
		{"decimal zero scale", Value{Kind: ValueDecimal, Text: "0.00000000"}, "0.00000000"},
		{"decimal exponent", Value{Kind: ValueDecimal, Text: "1.2300E-8"}, "1.2300E-8"},
		{"double", decimalAsDouble(Decimal(0.5)), "0.5"},
		{"boolean", Bool(true), "true"},
		{"empty string", String(""), ""},
		{"string", String("héllo"), "héllo"},
		{"empty list", List(), "List[]"},
		{"list", List(Int(1), Null, String("a")), "List[1, null, a]"},
		{"nested list", List(List(Int(1)), List()), "List[List[1], List[]]"},
		{"set", Set(String("a"), Int(2)), "Set[a, 2]"},
		{"empty map", Map(), "{}"},
		{"map", numbers, "{a=(1, 2), b=2}"},
		{"map object key", keyed, "{Example:{name=sample}=v}"},
		{"sobject", record, "Account:{Id=001000000000001AAA, Name=Acme}"},
		{"project class", project, "Widget:[count=3, label=w]"},
		{"empty project class", emptyProject, "Empty:[]"},
		{"object fields", plain, "Example:{name=sample}"},
		{"object value omits fields", valueObject, "short"},
		{"page reference omits fields", page, "/short"},
		{"page reference without url", pageNoURL, ""},
		{"sobject type", sobjectType, "Account"},
		{"sobject field", sobjectField, "Name"},
		{"message omits fields", message, "went wrong"},
		{"query locator", locator, "Database.QueryLocator[Query=SELECT Id FROM Account]"},
		{"dml options", options, "DMLOptions:[AllowFieldTruncation=null, AssignmentRuleHeader=AssignmentRuleHeader:[AssignmentRuleId=null, UseDefaultRule=true], DuplicateRuleHeader=null, EmailHeader=null, LocaleOptions=null, LocalizeErrors=null, OptAllOrNone=true]"},
		{"lead convert", convert, "Database.LeadConvert[getAccountId=null;getAccountRecord=null;getBypassAccountDedupeCheck=null;getBypassContactDedupeCheck=null;getContactId=null;getContactRecord=null;getConvertedStatus=Closed;getLeadId=null;getOpportunityId=null;getOpportunityName=null;getOpportunityRecord=null;getOwnerId=null;getRelatedPersonAccountId=null;getRelatedPersonAccountRecord=null;isDoNotCreateOpportunity=false;isOverwriteLeadSource=false;isSendNotificationEmail=false;]"},
		{"invocable action", action, "Action:[apiVersion=null, invocations=(1), invokeFromLightningComponent=null, name=act, namespace=null, type=null, version=null]"},
		{"location", location, "System.Location[getLatitude=1.0;getLongitude=2.0;]"},
		{"access level", access, "AccessLevel:[SYSTEM_MODE=true, USER_MODE=<>, currentAccessPermissions=<>, permSetId=null]"},
		{"stub proxy", stub, "Widget__sfdc_ApexStub:7"},
		{"http request", request, "System.HttpRequest[Endpoint=https://example.test, Method=GET]"},
		{"messaging template", template, "Messaging.RenderEmailTemplateBodyResult[getErrors=List[];getMergedBody=body;getSuccess=true;]"},
		{"cyclic object", selfRef, "Node:{name=n, next=Node:{...}}"},
		{"nested omitted fields", List(valueObject, page, List(message)), "List[short, /short, List[went wrong]]"},
		{"empty object", Object("Thing"), "Thing:{}"},
		{"over bound string", String(big), big},
		{"over bound list", List(String(big), Int(1)), "List[" + big + ", 1]"},
	}
}

// TestBoundedStringMatchesString checks that the bounded renderer is the
// String renderer: exact text when it fits, otherwise an exact prefix.
func TestBoundedStringMatchesString(t *testing.T) {
	for _, c := range valueTextCases() {
		t.Run(c.name, func(t *testing.T) {
			want := c.value.String()
			if want != c.want {
				t.Fatalf("String = %q, want %q", want, c.want)
			}
			for _, limit := range []int{0, 1, 2, 7, 64, 255, 256, 257, 1000, len(want) - 1, len(want), len(want) + 1} {
				if limit < 0 {
					continue
				}
				got, truncated := c.value.BoundedString(limit)
				if len(want) <= limit {
					if got != want || truncated {
						t.Fatalf("limit %d: got %q truncated=%v, want exact %q", limit, got, truncated, want)
					}
				} else if got != want[:limit] || !truncated {
					t.Fatalf("limit %d: got %q truncated=%v, want prefix %q", limit, got, truncated, want[:limit])
				}
			}
		})
	}
}

func TestBoundedStringStopsCyclicList(t *testing.T) {
	list := List(Null)
	list.List[0] = list
	got, truncated := list.BoundedString(64)
	if !truncated || got != strings.Repeat("List[", 13)[:64] {
		t.Fatalf("got %q truncated=%v", got, truncated)
	}
}

func TestBoundedStringBoundsWork(t *testing.T) {
	text := String(strings.Repeat("x", 128<<10))
	shared := make([]Value, 63)
	for i := range shared {
		shared[i] = text
	}
	ints := make([]Value, 200000)
	for i := range ints {
		ints[i] = Int(int64(i))
	}
	keyed := Map()
	keyed.Map["k"] = Int(1)
	keyed.MapKeys["k"] = List(ints...)
	hugeMap := Map()
	hugeObject := Object("Huge")
	for i := 0; i < 200000; i++ {
		name := fmt.Sprintf("k%06d", i)
		hugeMap.Map[name] = Int(int64(i))
		hugeObject.Fields[name] = Int(int64(i))
	}
	deep := Int(1)
	for i := 0; i < 10000; i++ {
		deep = List(deep)
	}
	for name, value := range map[string]Value{
		"shared large strings": List(shared...),
		"many integers":        List(ints...),
		"large map key":        keyed,
		"huge map":             hugeMap,
		"huge object":          hugeObject,
		"huge map in list":     List(hugeMap),
		"deep nesting":         deep,
	} {
		t.Run(name, func(t *testing.T) {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			got, truncated := value.BoundedString(256)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			if !truncated || len(got) != 256 {
				t.Fatalf("got %d bytes truncated=%v", len(got), truncated)
			}
			if n := after.TotalAlloc - before.TotalAlloc; n > 1<<20 {
				t.Fatalf("allocated %d bytes", n)
			}
			if elapsed > time.Second {
				t.Fatalf("took %s", elapsed)
			}
		})
	}
}

func TestBoundedStringDecimalAndAccessLevelWork(t *testing.T) {
	decimal := Value{Kind: ValueDecimal, Text: "0." + strings.Repeat("0", 7) + strings.Repeat("1", 1<<20)}
	want := decimal.String()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, truncated := decimal.BoundedString(256)
	runtime.ReadMemStats(&after)
	if !truncated || got != want[:256] {
		t.Fatalf("decimal prefix mismatch")
	}
	if n := after.TotalAlloc - before.TotalAlloc; n > 1<<16 {
		t.Fatalf("decimal allocated %d bytes", n)
	}
	deep := Object("AccessLevel")
	for i := 0; i < 10000; i++ {
		next := Object("AccessLevel")
		next.Fields["SYSTEM_MODE"] = deep
		deep = next
	}
	got, truncated = deep.BoundedString(256)
	if !truncated || len(got) != 256 {
		t.Fatalf("access level: %d bytes, truncated=%v", len(got), truncated)
	}
}
