public class LpRtProbe {
    static List<String> out;
    static void row(String id, Object observation) { out.add(id + '|' + String.valueOf(observation)); }
    static String err(Exception e) {
        if (e instanceof DmlException) {
            DmlException d = (DmlException) e;
            return 'DmlException ' + d.getDmlType(0) + ' ' + d.getDmlFieldNames(0) + ' ' + d.getDmlMessage(0);
        }
        return e.getTypeName() + ' ' + e.getMessage();
    }
    public static List<String> run() {
        out = new List<String>();
        Account a = new Account(Name = 'LP R');
        try { a.put('RecordTypeId', null); row('R01 put null', 'ok'); } catch (Exception e) { row('R01 put null', err(e)); }
        try { row('R02 get', a.get('RecordTypeId')); } catch (Exception e) { row('R02 get', err(e)); }
        Map<String, Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
        row('R03 fields has RecordTypeId', fields.containsKey('RecordTypeId'));
        row('R03 fields has recordtypeid', fields.containsKey('recordtypeid'));
        if (fields.containsKey('RecordTypeId')) {
            Schema.DescribeFieldResult d = fields.get('RecordTypeId').getDescribe();
            row('R04 field describe', d.getType() + ' ' + d.getReferenceTo() + ' ' + d.getRelationshipName() + ' ' + d.isCreateable() + ' ' + d.isUpdateable() + ' ' + d.isNillable() + ' ' + d.isDefaultedOnCreate());
        }
        List<String> infos = new List<String>();
        for (Schema.RecordTypeInfo i : Schema.SObjectType.Account.getRecordTypeInfos()) {
            infos.add(i.getDeveloperName() + ':' + i.isAvailable() + ':' + i.isDefaultRecordTypeMapping() + ':' + i.isMaster() + ':' + i.isActive());
        }
        infos.sort();
        row('R05 infos', String.join(infos, ','));
        List<String> byName = new List<String>(Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().keySet());
        byName.sort();
        row('R06 infos by developer name', String.join(byName, ','));
        List<RecordType> types = [SELECT DeveloperName, Name, IsActive, SobjectType FROM RecordType WHERE SobjectType = 'Account' ORDER BY DeveloperName];
        List<String> typeRows = new List<String>();
        for (RecordType t : types) typeRows.add(t.DeveloperName + ':' + t.Name + ':' + t.IsActive);
        row('R07 RecordType rows', String.join(typeRows, ','));
        Savepoint sp = Database.setSavepoint();
        try {
            Account plain = new Account(Name = 'LP R1');
            insert plain;
            Account q = (Account) Database.query('SELECT RecordTypeId FROM Account WHERE Id = \'' + plain.Id + '\'');
            row('R08 insert without RecordTypeId', 'ok ' + (q.get('RecordTypeId') == null ? 'null' : 'set'));
        } catch (Exception e) { row('R08 insert without RecordTypeId', err(e)); }
        Database.rollback(sp);
        sp = Database.setSavepoint();
        try {
            Account explicitNull = new Account(Name = 'LP R2');
            explicitNull.put('RecordTypeId', null);
            insert explicitNull;
            row('R09 insert RecordTypeId null', 'ok');
        } catch (Exception e) { row('R09 insert RecordTypeId null', err(e)); }
        Database.rollback(sp);
        if (!types.isEmpty()) {
            sp = Database.setSavepoint();
            try {
                Id typeId = [SELECT Id FROM RecordType WHERE SobjectType = 'Account' AND DeveloperName = 'LpAcct'].Id;
                Account typed = new Account(Name = 'LP R3');
                typed.put('RecordTypeId', typeId);
                insert typed;
                Account q = (Account) Database.query('SELECT RecordType.DeveloperName FROM Account WHERE Id = \'' + typed.Id + '\'');
                row('R10 insert LpAcct', 'ok ' + q.getSObject('RecordType').get('DeveloperName'));
            } catch (Exception e) { row('R10 insert LpAcct', err(e)); }
            Database.rollback(sp);
        }
        try {
            List<SObject> rows = Database.query('SELECT Id, RecordTypeId, RecordType.DeveloperName FROM Account LIMIT 1');
            row('R11 dynamic SOQL', 'ok');
        } catch (Exception e) { row('R11 dynamic SOQL', err(e)); }
        Contact c = new Contact(LastName = 'LP');
        try { c.put('RecordTypeId', null); row('R12 contact put', 'ok'); } catch (Exception e) { row('R12 contact put', err(e)); }
        return out;
    }
}
