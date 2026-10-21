@IsTest private class SchemaNameProbeTest {
    private static void seed() {
        insert new Schema.EmailTemplate(Name='Probe template', DeveloperName='ProbeTemplate',
            FolderId=UserInfo.getUserId(), TemplateType='text', Subject='Probe', Body='Hello');
    }
    @IsTest static void localClassWins() {
        EmailTemplate value = new EmailTemplate();
        System.assertEquals('local', SchemaNameProbe.localLabel(value));
        Object boxed = value;
        Boolean rejected = false;
        try { Schema.EmailTemplate invalid = (Schema.EmailTemplate)boxed; }
        catch (TypeException e) { rejected = true; }
        System.assert(rejected);
    }
    @IsTest static void querySingleton() {
        seed();
        Schema.EmailTemplate row = [SELECT Id, DeveloperName FROM EmailTemplate LIMIT 1];
        System.assertEquals('ProbeTemplate', SchemaNameProbe.recordName(row));
        Object boxed = row;
        System.assertEquals('ProbeTemplate', SchemaNameProbe.recordName((Schema.EmailTemplate)boxed));
        System.assertEquals('ProbeTemplate', SchemaNameProbe.recordFromObject(boxed));
    }
    @IsTest static void queryList() {
        seed();
        List<Schema.EmailTemplate> rows = [SELECT Id, DeveloperName FROM EmailTemplate];
        System.assertEquals(1, SchemaNameProbe.recordCount(rows));
        System.assertEquals(1, SchemaNameProbe.recordCount(SchemaNameProbe.records()));
        Object boxed = rows;
        System.assertEquals(1, SchemaNameProbe.recordCount((List<Schema.EmailTemplate>)boxed));
        System.assertEquals(1, SchemaNameProbe.recordsFromObject(boxed));
        System.assertEquals(1, SchemaNameProbe.recordCount(Database.query('SELECT Id FROM EmailTemplate')));
    }
    @IsTest static void emptyQueryList() {
        List<Schema.EmailTemplate> rows = [SELECT Id FROM EmailTemplate];
        System.assertEquals(0, SchemaNameProbe.recordCount(rows));
        System.assertEquals(0, SchemaNameProbe.recordCount(SchemaNameProbe.records()));
        Object boxed = rows;
        System.assertEquals(0, SchemaNameProbe.recordCount((List<Schema.EmailTemplate>)boxed));
        System.assertEquals(0, SchemaNameProbe.recordsFromObject(boxed));
    }
    @IsTest static void localClassListStaysDistinct() {
        Object boxed = new List<EmailTemplate>{new EmailTemplate()};
        Boolean rejected = false;
        try { List<Schema.EmailTemplate> invalid = (List<Schema.EmailTemplate>)boxed; }
        catch (TypeException e) { rejected = true; }
        System.assert(rejected);
        boxed = new List<EmailTemplate>();
        rejected = false;
        try { List<Schema.EmailTemplate> invalid = (List<Schema.EmailTemplate>)boxed; }
        catch (TypeException e) { rejected = true; }
        System.assert(rejected);
    }
}
