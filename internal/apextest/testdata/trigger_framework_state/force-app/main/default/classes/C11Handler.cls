public class C11Handler {
    public static void beforeInsert(List<C11Rec__c> recs) {
        Map<String, Schema.RecordTypeInfo> rts = Schema.SObjectType.C11Rec__c.getRecordTypeInfosByDeveloperName();
        for (C11Rec__c r : recs) {
            if (r.Kind__c == 'nested') {
                insert new C11Rec__c(Name = 'inner', Kind__c = 'bad');
            }
            C11Cfg__c cfg = C11Cfg__c.getInstance(String.valueOf(r.Kind__c));
            if (cfg == null) {
                throw new C11Exception('no mapping for ' + r.Kind__c);
            }
            r.RecordTypeId = rts.get(cfg.RtName__c).getRecordTypeId();
        }
    }
}
