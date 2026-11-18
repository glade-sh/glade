public without sharing class FernBump {
    @InvocableMethod
    public static void run(List<Fern_Rec__c> rows) {
        FernLog.mark('Bump', rows);
        List<Fern_Rec__c> fresh = new List<Fern_Rec__c>();
        for (Fern_Rec__c r : rows) { fresh.add(new Fern_Rec__c(Id = r.Id, Score__c = (r.Score__c == null ? 0 : r.Score__c) + 1)); }
        update fresh;
    }
}
