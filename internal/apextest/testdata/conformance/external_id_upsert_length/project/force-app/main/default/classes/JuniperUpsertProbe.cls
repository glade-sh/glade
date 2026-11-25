public class JuniperUpsertProbe {
    public static Map<String, String> observe() {
        Map<String, String> rows = new Map<String, String>();
        rows.put('long-key-single', shape(Database.upsert(
            new Juniper_Mark__c(Name='Single', Mark_Key__c='K12345678'),
            Juniper_Mark__c.Fields.Mark_Key__c, false)));

        Database.UpsertResult[] batch = Database.upsert(new List<Juniper_Mark__c>{
            new Juniper_Mark__c(Name='Batch long', Mark_Key__c='C12345678'),
            new Juniper_Mark__c(Name='Batch exact', Mark_Key__c='C1234567')
        }, Juniper_Mark__c.Fields.Mark_Key__c, false);
        rows.put('long-key-batch', shape(batch[0]));
        rows.put('max-key-batch', shape(batch[1]));

        rows.put('long-nonkey', shape(Database.upsert(
            new Juniper_Mark__c(Name='Long note', Mark_Key__c='N1', Note__c='N12345678'),
            Juniper_Mark__c.Fields.Mark_Key__c, false)));
        rows.put('long-key-implicit-id', shape(Database.upsert(
            new Juniper_Mark__c(Name='Implicit selector', Mark_Key__c='I12345678'), false)));
        rows.put('missing-required-default-upsert', shape(Database.upsert(new Account(), false)));
        rows.put('padded-max-key', shape(Database.upsert(
            new Juniper_Mark__c(Name='Padded', Mark_Key__c=' P1234567 '),
            Juniper_Mark__c.Fields.Mark_Key__c, false)));
        rows.put('supplementary-key', shape(Database.upsert(
            new Juniper_Mark__c(Name='Supplementary', Mark_Key__c='J123456🚀'),
            Juniper_Mark__c.Fields.Mark_Key__c, false)));
        rows.put('long-key-and-nonkey', shape(Database.upsert(
            new Juniper_Mark__c(Name='Both long', Mark_Key__c='W12345678', Note__c='W12345678'),
            Juniper_Mark__c.Fields.Mark_Key__c, false)));

        Juniper_Mark__c requestedTruncation = new Juniper_Mark__c(Name='Truncate', Mark_Key__c='T12345678');
        Database.DMLOptions options = new Database.DMLOptions();
        options.allowFieldTruncation = true;
        requestedTruncation.setOptions(options);
        rows.put('long-key-truncation-option', shape(Database.upsert(
            requestedTruncation, Juniper_Mark__c.Fields.Mark_Key__c, false)));
        return rows;
    }

    private static String shape(Database.UpsertResult row) {
        String error = '';
        if (!row.getErrors().isEmpty()) {
            Database.Error firstError = row.getErrors()[0];
            error = String.valueOf(firstError.getStatusCode()) + ':' + String.join(firstError.getFields(), ',');
            if (firstError.getStatusCode() == StatusCode.FIELD_INTEGRITY_EXCEPTION) {
                error += '|' + firstError.getMessage();
            }
        }
        return row.isSuccess() + '|' + row.isCreated() + '|' + (row.getId() != null) + '|' + error;
    }
}
