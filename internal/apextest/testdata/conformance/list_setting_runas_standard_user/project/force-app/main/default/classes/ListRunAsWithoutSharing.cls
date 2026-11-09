public without sharing class ListRunAsWithoutSharing {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        try { m.put('count', [SELECT COUNT() FROM V35List__c]); } catch (Exception e) { m.put('count', e.getTypeName()); }
        try { m.put('queryByName', [SELECT Val__c FROM V35List__c WHERE Name = 'alpha'].size()); } catch (Exception e) { m.put('queryByName', e.getTypeName()); }
        return m;
    }
    public static Map<String,Object> observeHier() {
        Map<String,Object> m = new Map<String,Object>();
        try { m.put('count', [SELECT COUNT() FROM V35Hier__c]); } catch (Exception e) { m.put('count', e.getTypeName()); }
        try { m.put('queryByVal', [SELECT Id FROM V35Hier__c WHERE Val__c = 'a'].size()); } catch (Exception e) { m.put('queryByVal', e.getTypeName()); }
        return m;
    }
}
