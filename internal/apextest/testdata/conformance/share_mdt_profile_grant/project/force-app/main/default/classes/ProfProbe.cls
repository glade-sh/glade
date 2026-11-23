public class ProfProbe {
    public static String go() {
        Map<String,String> got = new Map<String,String>();
        try { Schema.DescribeSObjectResult d = Synth__mdt.SObjectType.getDescribe(); got.put('desc', d.isAccessible() + ''); } catch (Exception e) { got.put('desc', e.getMessage()); }
        try { got.put('q', String.valueOf(Database.query('SELECT MasterLabel, DeveloperName FROM Synth__mdt WITH USER_MODE').size())); } catch (Exception e) { got.put('q', e.getTypeName() + '|' + e.getMessage()); }
        return JSON.serialize(got);
    }
}
