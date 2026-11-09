public without sharing class LaneHProbe {
    public static Map<String, Object> keysDiff(String resourceName) {
        Set<String> want = new Set<String>(new StaticResourceCalloutHelper().lines(resourceName));
        Set<String> have = Schema.getGlobalDescribe().keySet();
        List<String> added = new List<String>();
        List<String> removed = new List<String>();
        for (String k : have) if (!want.contains(k)) added.add(k);
        for (String k : want) if (!have.contains(k)) removed.add(k);
        added.sort(); removed.sort();
        return new Map<String, Object>{'count' => have.size(), 'notInAnon' => added, 'onlyInAnon' => removed};
    }
    public class StaticResourceCalloutHelper {
        public List<String> lines(String name) {
            StaticResource r = [SELECT Body FROM StaticResource WHERE Name = :name LIMIT 1];
            List<String> out = new List<String>();
            for (String s : r.Body.toString().split('\n')) if (s.trim() != '') out.add(s.trim());
            return out;
        }
    }

    static String names(List<SObject> rows, String nameField) {
        List<String> n = new List<String>();
        for (SObject r : rows) n.add(String.valueOf(r.get(nameField)));
        return String.join(n, ',');
    }

    // n1 holds the higher value (the later target), n2 the lower, n3 null.
    static void probe(Map<String, Object> out, String obj, String field, String nameField, Object high, Object low, Map<String, Object> base) {
        String key = obj + '.' + field;
        Savepoint sp = Database.setSavepoint();
        try {
            Schema.SObjectType t = Schema.getGlobalDescribe().get(obj);
            List<SObject> rows = new List<SObject>();
            List<Object> values = new List<Object>{high, low, null};
            for (Integer i = 0; i < 3; i++) {
                SObject r = t.newSObject();
                for (String b : base.keySet()) r.put(b, base.get(b));
                r.put(nameField, 'n' + (i + 1));
                r.put(field, values[i]);
                rows.add(r);
            }
            insert as system rows;
            Set<Id> ids = new Map<Id, SObject>(rows).keySet();
            Map<String, String> m = new Map<String, String>();
            for (String tail : new List<String>{'ASC', 'ASC NULLS FIRST', 'ASC NULLS LAST', 'DESC', 'DESC NULLS FIRST', 'DESC NULLS LAST'}) {
                m.put(tail, names(Database.query('SELECT Id, ' + nameField + ' FROM ' + obj + ' WHERE Id IN :ids ORDER BY ' + field + ' ' + tail + ', ' + nameField), nameField));
            }
            out.put(key, m);
        } catch (Exception e) {
            out.put(key, 'EXC|' + e.getTypeName() + '|' + e.getMessage());
        }
        Database.rollback(sp);
    }

    public static Map<String, Object> nullOrder(Integer part) {
        Map<String, Object> out = new Map<String, Object>();
        Account a1 = new Account(Name = 'LH a1'); insert as system a1;
        Account a2 = new Account(Name = 'LH a2'); insert as system a2;
        Contact k1 = new Contact(LastName = 'LH k1'); insert as system k1;
        Contact k2 = new Contact(LastName = 'LH k2'); insert as system k2;
        Campaign g1 = new Campaign(Name = 'LH g1'); insert as system g1;
        Campaign g2 = new Campaign(Name = 'LH g2'); insert as system g2;
        Case s1 = new Case(Subject = 'LH s1'); insert as system s1;
        Case s2 = new Case(Subject = 'LH s2'); insert as system s2;
        Asset t1 = new Asset(Name = 'LH t1', AccountId = a1.Id); insert as system t1;
        Asset t2 = new Asset(Name = 'LH t2', AccountId = a1.Id); insert as system t2;
        Map<String, Object> none = new Map<String, Object>();
        Map<String, Object> opp = new Map<String, Object>{'StageName' => 'Prospecting', 'CloseDate' => Date.newInstance(2026, 12, 1)};
        if (Math.mod(0, 3) == part) probe(out, 'Contact', 'AccountId', 'LastName', a2.Id, a1.Id, none);
        if (Math.mod(1, 3) == part) probe(out, 'Contact', 'ReportsToId', 'LastName', k2.Id, k1.Id, none);
        if (Math.mod(2, 3) == part) probe(out, 'Contact', 'V35H_Acc__c', 'LastName', a2.Id, a1.Id, none);
        if (Math.mod(3, 3) == part) probe(out, 'Contact', 'V35H_Ext__c', 'LastName', 'b', 'a', none);
        if (Math.mod(4, 3) == part) probe(out, 'Contact', 'V35H_Txt__c', 'LastName', 'b', 'a', none);
        if (Math.mod(5, 3) == part) probe(out, 'Contact', 'Email', 'LastName', 'b@example.invalid', 'a@example.invalid', none);
        if (Math.mod(6, 3) == part) probe(out, 'Contact', 'Title', 'LastName', 'b', 'a', none);
        if (Math.mod(7, 3) == part) probe(out, 'Account', 'ParentId', 'Name', a2.Id, a1.Id, none);
        if (Math.mod(8, 3) == part) probe(out, 'Account', 'Site', 'Name', 'b', 'a', none);
        if (Math.mod(9, 3) == part) probe(out, 'Campaign', 'ParentId', 'Name', g2.Id, g1.Id, none);
        if (Math.mod(10, 3) == part) probe(out, 'Case', 'AccountId', 'Subject', a2.Id, a1.Id, none);
        if (Math.mod(11, 3) == part) probe(out, 'Case', 'ContactId', 'Subject', k2.Id, k1.Id, none);
        if (Math.mod(12, 3) == part) probe(out, 'Case', 'ParentId', 'Subject', s2.Id, s1.Id, none);
        if (Math.mod(13, 3) == part) probe(out, 'Case', 'AssetId', 'Subject', t2.Id, t1.Id, none);
        if (Math.mod(14, 3) == part) probe(out, 'Case', 'SuppliedEmail', 'Subject', 'b@example.invalid', 'a@example.invalid', none);
        if (Math.mod(15, 3) == part) probe(out, 'Opportunity', 'AccountId', 'Name', a2.Id, a1.Id, opp);
        if (Math.mod(16, 3) == part) probe(out, 'Opportunity', 'CampaignId', 'Name', g2.Id, g1.Id, opp);
        if (Math.mod(17, 3) == part) probe(out, 'Asset', 'ContactId', 'Name', k2.Id, k1.Id, new Map<String, Object>{'AccountId' => a1.Id});
        if (Math.mod(18, 3) == part) probe(out, 'Asset', 'ParentId', 'Name', t2.Id, t1.Id, new Map<String, Object>{'AccountId' => a1.Id});
        if (Math.mod(19, 3) == part) probe(out, 'Task', 'WhatId', 'Subject', a2.Id, a1.Id, none);
        if (Math.mod(20, 3) == part) probe(out, 'Task', 'WhoId', 'Subject', k2.Id, k1.Id, none);
        if (Math.mod(21, 3) == part) probe(out, 'Event', 'WhatId', 'Subject', a2.Id, a1.Id, new Map<String, Object>{'DurationInMinutes' => 30, 'ActivityDateTime' => Datetime.newInstanceGmt(2026, 12, 1, 12, 0, 0)});
        if (Math.mod(22, 3) == part) probe(out, 'Lead', 'Email', 'LastName', 'b@example.invalid', 'a@example.invalid', new Map<String, Object>{'Company' => 'LH'});
        if (Math.mod(23, 3) == part) probe(out, 'Lead', 'Title', 'LastName', 'b', 'a', new Map<String, Object>{'Company' => 'LH'});
        return out;
    }

    public static Map<String, Object> privacy(String part) {
        Map<String, Object> out = new Map<String, Object>();
        Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
        out.put('present', g.containsKey('PrivacyRTBFRequest'));
        if (part == 'fields' && g.containsKey('PrivacyRTBFRequest')) {
            Schema.DescribeSObjectResult d = g.get('PrivacyRTBFRequest').getDescribe();
            out.put('keyPrefix', d.getKeyPrefix());
            out.put('label', d.getLabel());
            out.put('flags', 'c=' + d.isCreateable() + ',q=' + d.isQueryable() + ',u=' + d.isUpdateable() + ',d=' + d.isDeletable());
            List<String> fields = new List<String>();
            for (Schema.SObjectField f : d.fields.getMap().values()) {
                Schema.DescribeFieldResult x = f.getDescribe();
                fields.add(x.getName() + ':' + x.getType() + ':' + x.isRestrictedPicklist() + ':' + x.isCreateable() + ':' + x.isNillable());
            }
            fields.sort();
            out.put('fields', fields);
        }
        for (String s : new List<String>{part}) {
            if (!s.contains('.')) continue;
            List<String> p = s.split('\\.');
            List<String> refs = new List<String>();
            for (Schema.SObjectType t : g.get(p[0]).getDescribe().fields.getMap().get(p[1]).getDescribe().getReferenceTo()) refs.add(String.valueOf(t));
            refs.sort();
            out.put(s, refs);
        }
        return out;
    }

    public static Map<String, Object> picks() {
        Map<String, Object> out = new Map<String, Object>();
        Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
        for (String o : new List<String>{'Account', 'Contact', 'Lead', 'Case', 'Opportunity', 'AccountCleanInfo', 'ContactCleanInfo', 'ConsumptionScheduleShare', 'AccountShare', 'V35H_Pick__c'}) {
            if (!g.containsKey(o)) { out.put(o, 'absent'); continue; }
            List<String> fields = new List<String>();
            for (Schema.SObjectField f : g.get(o).getDescribe().fields.getMap().values()) {
                Schema.DescribeFieldResult x = f.getDescribe();
                if (x.getType() == Schema.DisplayType.PICKLIST || x.getType() == Schema.DisplayType.MULTIPICKLIST) fields.add(x.getName() + ':' + (x.isRestrictedPicklist() ? 'R' : 'U') + ':' + x.getPicklistValues().size());
            }
            fields.sort();
            out.put(o, fields);
        }
        return out;
    }
}
