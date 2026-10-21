public class RecentFieldProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> out = new Map<String,Object>();
        for(String objectName : new List<String>{'ViewedSample__c','SearchSample__c','BareSample__c'}) {
            Map<String,Schema.SObjectField> fields = Schema.getGlobalDescribe().get(objectName).getDescribe().fields.getMap();
            for(String fieldName : new List<String>{'LastViewedDate','LastReferencedDate'}) {
                String key = objectName+'.'+fieldName;
                Boolean present = fields.containsKey(fieldName);
                out.put(key+'.present',present);
                if(present) {
                    Schema.DescribeFieldResult info = fields.get(fieldName).getDescribe();
                    out.put(key+'.type',String.valueOf(info.getType()));
                    out.put(key+'.writable',info.isCreateable() || info.isUpdateable());
                }
                try {
                    List<SObject> rows = Database.query('SELECT '+fieldName+' FROM '+objectName+' LIMIT 1');
                    out.put(key+'.query',true);
                } catch(QueryException ex) { out.put(key+'.query',false); }
            }
        }
        return out;
    }
    public static void staticQueries() {
        List<ViewedSample__c> a = [SELECT LastViewedDate,LastReferencedDate FROM ViewedSample__c LIMIT 1];
        List<SearchSample__c> b = [SELECT LastViewedDate,LastReferencedDate FROM SearchSample__c LIMIT 1];
    }
}
