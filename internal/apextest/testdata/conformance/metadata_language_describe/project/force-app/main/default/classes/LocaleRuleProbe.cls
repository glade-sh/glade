public class LocaleRuleProbe {
    private static String facets(Schema.DescribeFieldResult field) {
        return field.getType() + ':' + field.getLength() + ':' + field.isGroupable() + ':' + field.isFilterable() + ':' + field.isSortable();
    }
    public static String describeStatic() { return facets(LocaleRule__mdt.Language.getDescribe()); }
    public static String describeDynamic() { return facets(LocaleRule__mdt.SObjectType.getDescribe().fields.getMap().get('Language').getDescribe()); }
    public static String queryLanguage() { return [SELECT Language FROM LocaleRule__mdt WHERE Language = 'en_US' ORDER BY Language LIMIT 1].Language; }
    public static String instanceLanguage() { return LocaleRule__mdt.getInstance('Alpha').Language; }
    public static String allLanguage() { return LocaleRule__mdt.getAll().get('Alpha').Language; }
    public static String constructedLanguage() { return new LocaleRule__mdt(Language='fr').Language; }
}
