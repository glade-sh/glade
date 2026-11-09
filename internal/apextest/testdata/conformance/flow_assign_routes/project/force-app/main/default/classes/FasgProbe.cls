public class FasgProbe {
    static String show(Object v) {
        if (v == null) {
            return 'null';
        }
        String s = String.valueOf(v);
        String other = '';
        for (Integer c : s.getChars()) {
            if (!((c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122))) {
                other += (other == '' ? '' : ',') + c;
            }
        }
        return 'len=' + s.length() + ' text=[' + s + '] other=[' + other + ']';
    }
    static List<String> accountRow(Id id) {
        Account a = [SELECT Fw1__c, Fw2__c, Fw3__c, Fw4__c, Fw5__c, Fw6__c FROM Account WHERE Id = :id];
        return new List<String>{ show(a.Fw1__c), show(a.Fw2__c), show(a.Fw3__c), show(a.Fw4__c), show(a.Fw5__c), show(a.Fw6__c) };
    }
    static Object insertAccount(String kind, String name) {
        Account a = new Account(Name = name, Kind__c = kind);
        try {
            insert a;
        } catch (DmlException e) {
            List<String> fields = e.getDmlFieldNames(0);
            return 'error:' + e.getDmlType(0) + ':' + show(e.getDmlMessage(0)) + ':' + String.join(fields, ',');
        }
        return accountRow(a.Id);
    }
    static List<String> contactRow(Id id) {
        Contact c = [SELECT Cw1__c, Cw2__c, Cw3__c, Cw4__c FROM Contact WHERE Id = :id];
        return new List<String>{ show(c.Cw1__c), show(c.Cw2__c), show(c.Cw3__c), show(c.Cw4__c) };
    }
    public static String sample() {
        Map<String, Object> out = new Map<String, Object>();
        out.put('preR1', insertAccount('FA', 'r1'));
        out.put('preDefault', insertAccount('FA', 'zz'));
        out.put('seqR1', insertAccount('FB', 'r1'));
        out.put('seqR2', insertAccount('FB', 'r2'));
        out.put('seqDefault', insertAccount('FB', 'zz'));
        out.put('valueSet', insertAccount('FC', 'zz'));
        out.put('loop', insertAccount('FD', 'zz'));
        out.put('afterR1', insertAccount('FE', 'r1'));
        out.put('afterDefault', insertAccount('FE', 'zz'));
        out.put('brAfter', insertAccount('FG', 'zz'));
        out.put('customError', insertAccount('FH', 'Abc'));
        out.put('customErrorBr', insertAccount('FI', 'zz'));
        Account parent = new Account(Name = 'Parent');
        insert parent;
        Contact before = new Contact(LastName = 'c1', AccountId = parent.Id, CKind__c = 'FJ');
        Contact orphan = new Contact(LastName = 'c2', CKind__c = 'FJ');
        Contact after = new Contact(LastName = 'c3', AccountId = parent.Id, CKind__c = 'FK');
        Contact afterOrphan = new Contact(LastName = 'c4', CKind__c = 'FK');
        insert new List<Contact>{ before, orphan, after, afterOrphan };
        out.put('crossBefore', contactRow(before.Id));
        out.put('crossBeforeOrphan', contactRow(orphan.Id));
        out.put('crossAfter', contactRow(after.Id));
        out.put('crossAfterOrphan', contactRow(afterOrphan.Id));
        return JSON.serialize(out);
    }
}
