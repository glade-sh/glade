public class MetadataFieldProbe {
    public static String language() {
        CaptureRule__mdt value = CaptureRule__mdt.getInstance('Alpha');
        return value.Language + ':' + value.DeveloperName;
    }
    public static String populated() { return flags(CaptureRule__mdt.getInstance('Alpha')); }
    public static String allPopulated() { return flags(CaptureRule__mdt.getAll().get('Alpha')); }
    private static String flags(CaptureRule__mdt value) {
        Map<String,Object> fields = value.getPopulatedFieldsAsMap();
        return fields.containsKey('Id') + ':' + fields.containsKey('SystemModstamp') + ':' + fields.containsKey('Language') + ':' + (fields.get('Id') == value.Id) + ':' + (value.SystemModstamp != null) + ':' + (fields.get('SystemModstamp') == value.SystemModstamp) + ':' + (fields.get('SystemModstamp') instanceof Datetime);
    }
    public static String constructed() {
        CaptureRule__mdt value = new CaptureRule__mdt(Language='en_US');
        Map<String,Object> fields = value.getPopulatedFieldsAsMap();
        return fields.containsKey('Id') + ':' + fields.containsKey('SystemModstamp') + ':' + fields.containsKey('Language') + ':' + (value.Id == null) + ':' + (value.SystemModstamp == null);
    }
    public static String queryFields() {
        List<CaptureRule__mdt> values = [SELECT Id, DeveloperName, Language, Label, MasterLabel, QualifiedApiName, NamespacePrefix, SystemModstamp FROM CaptureRule__mdt WHERE DeveloperName = 'Alpha' AND Language = 'en_US' ORDER BY Language];
        return values.size() + ':' + values[0].Language;
    }
}
