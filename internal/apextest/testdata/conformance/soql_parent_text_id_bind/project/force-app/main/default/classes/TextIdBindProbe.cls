public class TextIdBindProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        Savepoint sp = Database.setSavepoint();
        Account tpl = new Account(Name = 'tx tpl'); insert tpl;
        Account rel = new Account(Name = 'tx rel'); insert rel;
        ReqTx__c r = new ReqTx__c(Name = 'r', Tpl__c = tpl.Id, Rel__c = rel.Id); insert as system r;
        SgnTx__c s = new SgnTx__c(Name = 's', Req__c = r.Id, Email__c = 'a@b.com'); insert as system s;
        Id relId = rel.Id;
        String rel18 = String.valueOf(relId);
        String rel15 = rel18.substring(0, 15);
        Set<Id> relIds = new Set<Id>{ relId };
        List<String> relStrings = new List<String>{ rel18 };
        o.put('directIdBind', [SELECT Id FROM ReqTx__c WHERE Rel__c = :relId WITH SYSTEM_MODE].size());
        o.put('direct18', [SELECT Id FROM ReqTx__c WHERE Rel__c = :rel18 WITH SYSTEM_MODE].size());
        o.put('direct15', [SELECT Id FROM ReqTx__c WHERE Rel__c = :rel15 WITH SYSTEM_MODE].size());
        o.put('parentIdBind', [SELECT Id FROM SgnTx__c WHERE Req__r.Rel__c = :relId WITH SYSTEM_MODE].size());
        o.put('parent18', [SELECT Id FROM SgnTx__c WHERE Req__r.Rel__c = :rel18 WITH SYSTEM_MODE].size());
        o.put('parent15', [SELECT Id FROM SgnTx__c WHERE Req__r.Rel__c = :rel15 WITH SYSTEM_MODE].size());
        o.put('parentInIds', [SELECT Id FROM SgnTx__c WHERE Req__r.Rel__c IN :relIds WITH SYSTEM_MODE].size());
        o.put('parentInStrings', [SELECT Id FROM SgnTx__c WHERE Req__r.Rel__c IN :relStrings WITH SYSTEM_MODE].size());
        o.put('parentNotIdBind', [SELECT Id FROM SgnTx__c WHERE Req__r.Rel__c != :relId WITH SYSTEM_MODE].size());
        o.put('dynamicParentIdBind', Database.query('SELECT Id FROM SgnTx__c WHERE Req__r.Rel__c = :relId', AccessLevel.SYSTEM_MODE).size());
        o.put('combined', [SELECT Id FROM SgnTx__c WHERE Email__c = 'a@b.com' AND Req__r.Tpl__c = :tpl.Id AND Req__r.Rel__c = :relId WITH SYSTEM_MODE LIMIT 1].size());
        Database.rollback(sp);
        return o;
    }
}
