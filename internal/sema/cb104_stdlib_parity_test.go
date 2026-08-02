package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

type cb104ParityRow struct {
	surfaceID string
	accepted  bool
	source    string
}

// These are the exact call shapes from CB101's accepted replay and CB78/CB79/
// CB82's rejected sources. Keep one row per frozen surface so this table is a
// compile-direction regression gate, not a broad representative sample.
func TestCB104StdlibExactCompileParity(t *testing.T) {
	rows := []cb104ParityRow{
		{surfaceID: "apex:Approval.isLocked(Id)", accepted: true, source: `Object observed = Approval.isLocked('001000000000001AAA');`},
		{surfaceID: "apex:Approval.isLocked(List<Id>)", accepted: true, source: `Object observed = Approval.isLocked(new List<Id>());`},
		{surfaceID: "apex:Approval.lock(Id,Boolean)", accepted: true, source: `Object observed = Approval.lock('001000000000001AAA', false);`},
		{surfaceID: "apex:Approval.lock(Id)", accepted: true, source: `Object observed = Approval.lock('001000000000001AAA');`},
		{surfaceID: "apex:Approval.lock(List<Id>,Boolean)", accepted: true, source: `Object observed = Approval.lock(new List<Id>(), false);`},
		{surfaceID: "apex:Approval.lock(List<Id>)", accepted: true, source: `Object observed = Approval.lock(new List<Id>());`},
		{surfaceID: "apex:Approval.unlock(Id,Boolean)", accepted: true, source: `Object observed = Approval.unlock('001000000000001AAA', false);`},
		{surfaceID: "apex:Approval.unlock(Id)", accepted: true, source: `Object observed = Approval.unlock('001000000000001AAA');`},
		{surfaceID: "apex:Approval.unlock(List<Id>,Boolean)", accepted: true, source: `Object observed = Approval.unlock(new List<Id>(), false);`},
		{surfaceID: "apex:Approval.unlock(List<Id>)", accepted: true, source: `Object observed = Approval.unlock(new List<Id>());`},
		{surfaceID: "apex:System.Database.getCursor(String,Object)", accepted: true, source: `Object observed = Database.getCursor('cb70', (Object)null);`},
		{surfaceID: "apex:System.Database.getPaginationCursor(String,Object)", accepted: true, source: `Object observed = Database.getPaginationCursor('cb70', (Object)null);`},
		{surfaceID: "apex:System.InvalidParameterValueException.InvalidParameterValueException(String,String)", accepted: true, source: `Object observed = new InvalidParameterValueException('cb70', 'cb70');`},
		{surfaceID: "apex:System.List.equals(Object)", accepted: true, source: `Object observed = ((List<Object>)null).equals((Object)null);`},
		{surfaceID: "apex:System.Set.equals(Object)", accepted: true, source: `Object observed = ((Set<Object>)null).equals((Object)null);`},
		{surfaceID: "apex:System.Site.createExternalUser(Object,String,String,Boolean)", accepted: true, source: `Object observed = Site.createExternalUser(new Account(), 'cb70', 'cb70', false);`},
		{surfaceID: "apex:System.Site.createExternalUser(Object,String,String)", accepted: true, source: `Object observed = Site.createExternalUser(new Account(), 'cb70', 'cb70');`},
		{surfaceID: "apex:System.Site.createExternalUser(Object,String)", accepted: true, source: `Object observed = Site.createExternalUser(new Account(), 'cb70');`},
		{surfaceID: "apex:System.Site.createPortalUser(Object,String,String,Boolean)", accepted: true, source: `Object observed = Site.createPortalUser(new Account(), 'cb70', 'cb70', false);`},
		{surfaceID: "apex:System.Site.forgotPassword(String,String)", accepted: true, source: `Object observed = Site.forgotPassword('cb70', 'cb70');`},
		{surfaceID: "apex:System.System.equals(Object,Object)", accepted: true, source: `Object observed = System.equals((Object)null, (Object)null);`},

		{surfaceID: "apex:System.Assert.areEqual", source: `Assert.areEqual((Object)null, (Object)null, (Object)null);`},
		{surfaceID: "apex:System.Assert.areNotEqual", source: `Assert.areNotEqual((Object)null, (Object)null, (Object)null);`},
		{surfaceID: "apex:System.Assert.fail", source: `Assert.fail((Object)null);`},
		{surfaceID: "apex:System.Assert.isFalse", source: `Assert.isFalse(false, (Object)null);`},
		{surfaceID: "apex:System.Assert.isNotNull", source: `Assert.isNotNull((Object)null, (Object)null);`},
		{surfaceID: "apex:System.Assert.isNull", source: `Assert.isNull((Object)null, (Object)null);`},
		{surfaceID: "apex:System.Assert.isTrue", source: `Assert.isTrue(false, (Object)null);`},
		{surfaceID: "apex:System.Datetime.formatGmt", source: `Object observed = ((Datetime)null).formatGmt();`},
		{surfaceID: "apex:System.Decimal.valueOf", source: `Object observed = Decimal.valueOf(0.0);`},
		{surfaceID: "apex:System.Id.to18", source: `Id value = Id.valueOf('001B000001DVM9t'); String result = value.to18();`},
		{surfaceID: "apex:System.Integer.doubleValue", source: `Integer value = 42; Double result = value.doubleValue();`},
		{surfaceID: "apex:System.Map.containsValue", source: `Map<String,Integer> values = new Map<String,Integer>(); values.put('a', 1); Boolean result = values.containsValue(1);`},
		{surfaceID: "apex:System.Math.pow", source: `Object observed = Math.pow(0.0, 0.0);`},
		{surfaceID: "apex:System.Set.deepClone", source: `Set<Account> values = new Set<Account>(); Set<Account> result = values.deepClone();`},
		{surfaceID: "apex:System.String.commonPrefix", source: `String value = 'interstate'; String result = value.commonPrefix('interstellar');`},
		{surfaceID: "apex:System.String.escapeXml10", source: `String value = '<x>&'; String result = value.escapeXml10();`},
		{surfaceID: "apex:System.String.escapeXml11", source: `String value = '<x>&'; String result = value.escapeXml11();`},
		{surfaceID: "apex:System.String.join", source: `Object observed = String.join((Object)null, 'cb79');`},
		{surfaceID: "apex:System.String.lastIndexOfAny", source: `String value = 'abΩcdΩef'; Integer result = value.lastIndexOfAny('Ωf');`},
		{surfaceID: "apex:System.String.lastOrdinalIndexOf", source: `String value = 'one fish two fish red fish'; Integer result = value.lastOrdinalIndexOf('two', 1);`},
		{surfaceID: "apex:System.String.ordinalIndexOf", source: `String value = 'one fish two fish red fish'; Integer result = value.ordinalIndexOf('fish', 2);`},
		{surfaceID: "apex:System.String.removeIgnoreCase", source: `String value = 'Force FORCE force'; String result = value.removeIgnoreCase('force');`},
		{surfaceID: "apex:System.String.replaceIgnoreCase", source: `String value = 'Force FORCE force'; String result = value.replaceIgnoreCase('force', 'Cloud');`},
		{surfaceID: "apex:System.String.replaceOnce", source: `String value = 'Force FORCE force'; String result = value.replaceOnce('FORCE ', '');`},
		{surfaceID: "apex:System.String.rotate", source: `String value = 'abcdef'; String result = value.rotate(-2);`},
		{surfaceID: "apex:System.String.strip", source: `String value = '  abc  '; String result = value.strip();`},
		{surfaceID: "apex:System.String.stripAll", source: `List<String> values = new List<String>{'  one  ', '  two'}; List<String> result = String.stripAll(values);`},
		{surfaceID: "apex:System.String.stripEnd", source: `String value = '  abc  '; String result = value.stripEnd();`},
		{surfaceID: "apex:System.String.stripStart", source: `String value = '  abc  '; String result = value.stripStart();`},
		{surfaceID: "apex:System.String.stripToEmpty", source: `String value = '   '; String result = value.stripToEmpty();`},
		{surfaceID: "apex:System.String.stripToNull", source: `String value = '   '; String result = value.stripToNull();`},
		{surfaceID: "apex:System.String.unescapeXml10", source: `String value = '&lt;x&gt;'; String result = value.unescapeXml10();`},
		{surfaceID: "apex:System.String.unescapeXml11", source: `String value = '&lt;x&gt;'; String result = value.unescapeXml11();`},
	}

	accepted, rejected := 0, 0
	for _, row := range rows {
		if row.accepted {
			accepted++
		} else {
			rejected++
		}
		t.Run(row.surfaceID, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, row.source)
			if result.HasErrors() == !row.accepted {
				return
			}
			if row.accepted {
				t.Fatalf("Salesforce-accepted source rejected: %s: %#v", row.source, result.Diagnostics)
			}
			t.Fatalf("Salesforce-rejected source accepted: %s", row.source)
		})
	}
	if accepted != 21 || rejected != 33 {
		t.Fatalf("CB104 table counts = accepted %d, rejected %d; want 21 and 33", accepted, rejected)
	}
}
