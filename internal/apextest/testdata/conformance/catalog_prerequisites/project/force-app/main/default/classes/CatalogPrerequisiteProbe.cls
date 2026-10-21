public class CatalogPrerequisiteProbe {
    public class NestedProbeException extends Exception {}
    public static String sample() {
        Product2 entry = new Product2(Name = 'Synthetic inventory', IsActive = true);
        insert as system entry;
        Product2 stored = [SELECT Segment__c FROM Product2 WHERE Id = :entry.Id WITH SYSTEM_MODE];
        Integer matched = [SELECT COUNT() FROM Product2 WHERE Id = :entry.Id AND Segment__c = 'Group A' WITH SYSTEM_MODE];
        Map<String,Object> observed = new Map<String,Object>{'unsetPicklist' => stored.Segment__c, 'filteredCount' => matched, 'exceptionText' => String.valueOf(new NestedProbeException('synthetic failure')), 'orderCountry' => Schema.SObjectType.Order.fields.getMap().containsKey('ShippingCountryCode'), 'accountCountry' => Schema.SObjectType.Account.fields.getMap().containsKey('BillingCountryCode')};
        delete as system entry;
        return JSON.serialize(observed);
    }
}
