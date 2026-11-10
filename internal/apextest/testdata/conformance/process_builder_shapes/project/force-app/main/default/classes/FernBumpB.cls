public without sharing class FernBumpB {
    @InvocableMethod
    public static void run(List<Fern_Bmp__c> rows) {
        FernLog.mark('BumpB', rows);
        List<Fern_Bmp__c> fresh = new List<Fern_Bmp__c>();
        for (Fern_Bmp__c r : rows) { fresh.add(new Fern_Bmp__c(Id = r.Id, Score__c = (r.Score__c == null ? 0 : r.Score__c) + 1)); }
        update fresh;
    }
}
