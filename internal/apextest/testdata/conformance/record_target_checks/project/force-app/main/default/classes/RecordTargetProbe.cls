public class RecordTargetProbe {
    public Schema.SObjectType token;
    public RecordTargetProbe(Schema.SObjectType value) { token = value; }
    public static Schema.SObjectType echo(Schema.SObjectType value) { return value; }
    public static Map<String, String> observe() {
        Map<String, String> rows = new Map<String, String>();
        Schema.SObjectType direct = Schema.Account.SObjectType;
        RecordTargetProbe box = new RecordTargetProbe(echo(direct));
        rows.put('tokenArgumentReturnField', String.valueOf(box.token == direct));
        List<Schema.SObjectType> tokens = new List<Schema.SObjectType>{direct};
        Map<String,Schema.SObjectType> mapped = new Map<String,Schema.SObjectType>{'record'=>tokens[0]};
        rows.put('tokenCollection', String.valueOf(mapped.get('record') == direct));
        Schema.DescribeSObjectResult description = box.token.getDescribe();
        Map<String,Schema.SObjectField> fields = description.fields.getMap();
        rows.put('fieldToken', fields.get('Id').getDescribe().getName());
        rows.put('describeResult', description.getName());
        Account instance = new Account();
        Account copied = instance;
        Object classView = copied;
        rows.put('classObject', String.valueOf(classView instanceof Account) + ':' + String.valueOf(classView instanceof Schema.Account));
        try { Schema.Account rejected = (Schema.Account) classView; rows.put('classRecordCast', 'accepted'); }
        catch (TypeException error) { rows.put('classRecordCast', error.getTypeName()); }
        Schema.Account record = new Schema.Account(Name='record');
        Object objectView = record;
        SObject recordView = record;
        Schema.Account restored = (Schema.Account) objectView;
        rows.put('recordObject', String.valueOf(objectView instanceof Schema.Account) + ':' + recordView.get('Name') + ':' + restored.Name);
        try { Account rejected = (Account) objectView; rows.put('recordClassCast', 'accepted'); }
        catch (TypeException error) { rows.put('recordClassCast', error.getTypeName()); }
        return rows;
    }
    public static void emit() {
        Map<String,String> rows = observe();
        for (String key : new List<String>{'tokenArgumentReturnField','tokenCollection','fieldToken','describeResult','classObject','classRecordCast','recordObject','recordClassCast'}) {
            System.debug('ROW|' + key + '|' + rows.get(key));
        }
    }
}
