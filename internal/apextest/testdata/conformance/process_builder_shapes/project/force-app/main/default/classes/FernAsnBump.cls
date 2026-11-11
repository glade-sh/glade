public without sharing class FernAsnBump {
    @InvocableMethod
    public static void run(List<Fern_Asn__c> rows) {
        FernLog.mark('AsnAsf', rows);
        List<Fern_Asn__c> fresh = new List<Fern_Asn__c>();
        for (Fern_Asn__c r : rows) { fresh.add(new Fern_Asn__c(Id = r.Id, Score__c = (r.Score__c == null ? 0 : r.Score__c) + 1)); }
        update fresh;
    }
}
