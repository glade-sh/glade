public without sharing class DootAsyncWire67C26302350 {
    public static String payload(Map<String,Object> data) {
        String value=JSON.serialize(data);
        if(value.length()>620) return JSON.serialize(new Map<String,Object>{
            'v'=>1,'status'=>'inconclusive_payload_too_long','chars'=>value.length()});
        return value;
    }
    public static void error(Map<String,Object> data, Exception e, String stage) {
        data.put('error',new Map<String,Object>{'stage'=>stage,
            'type'=>e.getTypeName(),'message'=>e.getMessage()});
    }
    public static Map<String,Object> getter(String marker, String which) {
        Map<String,Object> data=new Map<String,Object>{'v'=>1,'cb'=>marker,'stage'=>'begin'};
        try {
            data.put('cap',which=='Y6'?Limits.getLimitQueueableJobs():Limits.getLimitQueries());
            data.put('stage','complete');
        } catch(Exception e) { error(data,e,'getter'); }
        return data;
    }
    public static Map<String,Object> relationship(String marker) {
        Map<String,Object> data=new Map<String,Object>{'v'=>1,'cb'=>marker,'stage'=>'begin'};
        String stage='snapshots';
        try {
            Integer q0=Limits.getQueries(),a0=Limits.getAggregateQueries(),r0=Limits.getQueryRows();
            stage='query';
            List<Account> rows=[SELECT Id,(SELECT Id FROM Contacts),
                (SELECT Id FROM Opportunities),(SELECT Id FROM Cases),
                (SELECT Id FROM Tasks) FROM Account WHERE Id=null WITH SYSTEM_MODE];
            Integer dq=Limits.getQueries()-q0,da=Limits.getAggregateQueries()-a0,
                dr=Limits.getQueryRows()-r0;
            stage='getters';
            data.put('q',dq); data.put('a',da); data.put('r',dr); data.put('n',rows.size());
            data.put('cap',Limits.getLimitAggregateQueries());
            data.put('qcap',Limits.getLimitQueries()); data.put('stage','complete');
        } catch(Exception e) { error(data,e,stage); }
        return data;
    }
    public static void persist(Id owned, Map<String,Object> data) {
        // Only the exact ID inserted by this test is ever updated.
        Account row=new Account(Id=owned,Description=payload(data));
        update as system row;
    }
    public static void read(Map<String,Object> data,Id owned,String expected) {
        String stage='ownedRead';
        try {
            List<Account> rows=Database.queryWithBinds(
                'SELECT Id,Description FROM Account WHERE Id=:owned',
                new Map<String,Object>{'owned'=>owned},AccessLevel.SYSTEM_MODE);
            data.put('ownedRows',rows.size());
            if(rows.size()!=1) { data.put('transport','inconclusive_owned_row_missing'); return; }
            stage='decode';
            Map<String,Object> cb=(Map<String,Object>)JSON.deserializeUntyped(rows[0].Description);
            data.put('cb',cb);
            if(cb.get('cb')!=expected) data.put('transport','inconclusive_callback_missing');
            else if(cb.get('stage')!='complete'||cb.containsKey('error'))
                data.put('transport','inconclusive_callback_incomplete');
            else data.put('transport','callback_complete');
        } catch(Exception e) { error(data,e,stage); data.put('transport','inconclusive_read'); }
    }
}
