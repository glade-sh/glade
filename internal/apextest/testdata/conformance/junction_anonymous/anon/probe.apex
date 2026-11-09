List<IspP1__c> p1 = new List<IspP1__c>{new IspP1__c(Name='P1 primary'), new IspP1__c(Name='P1 target')};
List<IspP2__c> p2 = new List<IspP2__c>{new IspP2__c(Name='P2 primary'), new IspP2__c(Name='P2 target')};
IspSeeder.seed(p1);
IspSeeder.seed(p2);
IspJ__c junction = new IspJ__c(Name='junction', P1__c=p1[0].Id, P2__c=p2[0].Id, Value__c='initial');
IspSeeder.seed(new List<SObject>{junction});
List<Object> out = new List<Object>();
for (String o : new List<String>{'IspJ__c','IspP1__c','IspP2__c'}) {
    out.add(new Map<String,Object>{'kind'=>'select','probe'=>o+'.select.with_sharing','rows'=>IspSharingReader.count(o)});
}
for (String o : new List<String>{'IspJ__c','IspP1__c','IspP2__c'}) {
    out.add(new Map<String,Object>{'kind'=>'select','probe'=>o+'.select.WITH_USER_MODE','rows'=>IspSharingReader.countUserMode(o)});
}
Set<Id> ids = new Set<Id>{junction.Id, p1[0].Id, p1[1].Id, p2[0].Id, p2[1].Id};
for (UserRecordAccess a : [SELECT RecordId, HasReadAccess, HasEditAccess, HasDeleteAccess, HasTransferAccess, HasAllAccess, MaxAccessLevel FROM UserRecordAccess WHERE UserId = :UserInfo.getUserId() AND RecordId IN :ids]) {
    out.add(new Map<String,Object>{'kind'=>'access','record'=>a.RecordId == junction.Id ? 'a00' : (new Set<Id>{p1[0].Id, p1[1].Id}.contains(a.RecordId) ? 'a01' : 'a02'),'read'=>a.HasReadAccess,'edit'=>a.HasEditAccess,'delete'=>a.HasDeleteAccess,'transfer'=>a.HasTransferAccess,'all'=>a.HasAllAccess,'level'=>a.MaxAccessLevel});
}
for (String op : new List<String>{'insert','scalar','reparent_p1','reparent_p2','reparent_both'}) {
    for (Boolean allOrNone : new List<Boolean>{false, true}) {
        IspJ__c fresh = new IspJ__c(Name='fresh', P1__c=p1[0].Id, P2__c=p2[0].Id, Value__c='initial');
        IspSeeder.seed(new List<SObject>{fresh});
        IspJ__c r;
        if (op == 'insert') r = new IspJ__c(Name='ISP insert probe', P1__c=p1[0].Id, P2__c=p2[0].Id, Value__c='inserted');
        else if (op == 'scalar') r = new IspJ__c(Id=fresh.Id, Value__c='changed');
        else if (op == 'reparent_p1') r = new IspJ__c(Id=fresh.Id, P1__c=p1[1].Id);
        else if (op == 'reparent_p2') r = new IspJ__c(Id=fresh.Id, P2__c=p2[1].Id);
        else r = new IspJ__c(Id=fresh.Id, P1__c=p1[1].Id, P2__c=p2[1].Id);
        Map<String,Object> m = new Map<String,Object>{'kind'=>'mutation','operation'=>op,'allOrNone'=>allOrNone};
        try {
            Database.SaveResult s = op == 'insert' ? Database.insert(r, allOrNone, AccessLevel.USER_MODE) : Database.update(r, allOrNone, AccessLevel.USER_MODE);
            m.put('outcome','SaveResult');
            m.put('success', s.isSuccess());
            List<Object> errs = new List<Object>();
            for (Database.Error e : s.getErrors()) errs.add(new Map<String,Object>{'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage(),'fields'=>e.getFields()});
            m.put('errors', errs);
        } catch (DmlException ex) {
            m.put('outcome','exception');
            m.put('message', ex.getMessage());
            m.put('numDml', ex.getNumDml());
            List<Object> errs = new List<Object>();
            for (Integer n = 0; n < ex.getNumDml(); n++) errs.add(new Map<String,Object>{'index'=>ex.getDmlIndex(n),'type'=>String.valueOf(ex.getDmlType(n)),'message'=>ex.getDmlMessage(n),'fields'=>ex.getDmlFieldNames(n),'hasId'=>ex.getDmlId(n)!=null});
            m.put('errors', errs);
        }
        out.add(m);
    }
}
System.debug(LoggingLevel.ERROR, 'JUNCTION_ANON|' + JSON.serialize(out));
