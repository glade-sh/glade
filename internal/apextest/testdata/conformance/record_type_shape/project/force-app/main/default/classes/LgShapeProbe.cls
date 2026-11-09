public class LgShapeProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        Map<String,Schema.SObjectType> g = Schema.getGlobalDescribe();
        Map<String,Object> std = new Map<String,Object>();
        for (String n : new List<String>{'Account','Contact','Opportunity','Case','Lead','Campaign','Task','Event','Contract','Order','Product2','Asset','Solution','Pricebook2','User','CampaignMember','OpportunityLineItem','Idea','V35NoRt__c','V35Rt__c'}) {
            Schema.SObjectType t = g.get(n);
            if (t == null) { std.put(n, 'absent'); continue; }
            Schema.DescribeSObjectResult d = t.getDescribe();
            List<String> infos = new List<String>();
            for (Schema.RecordTypeInfo i : d.getRecordTypeInfos()) { infos.add(i.getDeveloperName() + ':' + i.isAvailable() + ':' + i.isDefaultRecordTypeMapping() + ':' + i.isMaster()); }
            infos.sort();
            std.put(n, new Map<String,Object>{'rtField' => d.fields.getMap().containsKey('recordtypeid'), 'infos' => infos});
        }
        m.put('std', std);
        List<String> refs = new List<String>();
        Integer inGlobal = 0;
        for (Schema.SObjectType t : Task.WhatId.getDescribe().getReferenceTo()) { String n = t.getDescribe().getName(); refs.add(n); if (g.containsKey(n)) { inGlobal++; } }
        m.put('whatRefs', String.join(refs, ','));
        m.put('whatRefCount', refs.size());
        m.put('whatRefInGlobal', inGlobal);
        List<String> who = new List<String>();
        for (Schema.SObjectType t : Task.WhoId.getDescribe().getReferenceTo()) { who.add(t.getDescribe().getName()); }
        m.put('whoRefs', String.join(who, ','));
        List<String> ev = new List<String>();
        for (Schema.SObjectType t : Event.WhatId.getDescribe().getReferenceTo()) { ev.add(t.getDescribe().getName()); }
        m.put('eventWhatRefCount', ev.size());
        m.put('eventSameAsTask', String.join(ev, ',') == String.join(refs, ','));
        List<Integer> l10 = new List<Integer>(); for (Integer i = 1; i <= 10; i++) { l10.add(i); }
        List<Integer> l11 = new List<Integer>(); for (Integer i = 1; i <= 11; i++) { l11.add(i); }
        List<Integer> l101 = new List<Integer>(); for (Integer i = 1; i <= 101; i++) { l101.add(i); }
        m.put('list10', String.valueOf(l10));
        m.put('list11', String.valueOf(l11));
        m.put('list101', String.valueOf(l101).length());
        m.put('list101head', String.valueOf(l101).left(60));
        m.put('set11', String.valueOf(new Set<Integer>(l11)));
        List<String> s11 = new List<String>(); for (Integer i = 1; i <= 11; i++) { s11.add('s' + i); }
        m.put('strList11', String.valueOf(s11));
        List<Schema.SObjectType> t11 = new List<Schema.SObjectType>{Account.SObjectType, Contact.SObjectType, Case.SObjectType, Lead.SObjectType, Opportunity.SObjectType, Campaign.SObjectType, Contract.SObjectType, Order.SObjectType, Asset.SObjectType, Solution.SObjectType, Product2.SObjectType};
        m.put('typeList11', String.valueOf(t11));
        m.put('typeList10', String.valueOf(new List<Schema.SObjectType>{Account.SObjectType, Contact.SObjectType, Case.SObjectType, Lead.SObjectType, Opportunity.SObjectType, Campaign.SObjectType, Contract.SObjectType, Order.SObjectType, Asset.SObjectType, Solution.SObjectType}));
        m.put('debugList11', ('' + l11));
        try { List<SObject> rows = Database.query('SELECT RecordTypeId FROM Account LIMIT 1'); m.put('dynSoqlAccountRt', 'ok'); } catch (Exception e) { m.put('dynSoqlAccountRt', e.getTypeName() + ':' + e.getMessage()); }
        try { List<SObject> rows = Database.query('SELECT RecordTypeId FROM Contact LIMIT 1'); m.put('dynSoqlContactRt', 'ok'); } catch (Exception e) { m.put('dynSoqlContactRt', e.getTypeName() + ':' + e.getMessage()); }
        try { Contact c = new Contact(LastName = 'x'); m.put('contactGetRt', String.valueOf(c.get('RecordTypeId'))); } catch (Exception e) { m.put('contactGetRt', e.getTypeName() + ':' + e.getMessage()); }
        try { m.put('accountFieldToken', String.valueOf(Schema.getGlobalDescribe().get('Account').getDescribe().fields.getMap().get('RecordTypeId'))); } catch (Exception e) { m.put('accountFieldToken', e.getTypeName()); }
        return m;
    }
}
