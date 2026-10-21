public class SchemaNameProbe {
    public static String localLabel(EmailTemplate value) { return value.label; }
    public static String recordName(Schema.EmailTemplate value) { return value.DeveloperName; }
    public static Integer recordCount(List<Schema.EmailTemplate> values) { return values.size(); }
    public static String recordFromObject(Object value) {
        return ((Schema.EmailTemplate)value).DeveloperName;
    }
    public static Integer recordsFromObject(Object value) {
        return ((List<Schema.EmailTemplate>)value).size();
    }
    public static List<Schema.EmailTemplate> records() {
        return [SELECT Id, DeveloperName FROM EmailTemplate];
    }
}
