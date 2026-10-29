public class GapProbe {
    static Map<String, Object> err(Opportunity o) {
        Map<String, Object> r = new Map<String, Object>();
        try { insert o; r.put('exception', null); r.put('id', o.Id != null); }
        catch (DmlException e) { r.put('exception', e.getDmlStatusCode(0)); r.put('message', e.getDmlMessage(0)); r.put('fields', e.getDmlFieldNames(0).size()); }
        return r;
    }
    public static Map<String, Object> observe() {
        Map<String, Object> a = new Map<String, Object>();
        a.put('P01', err(new Opportunity(Name = 'ERR', StageName = 'Prospecting', CloseDate = Date.newInstance(2031, 3, 9), Amount = 1234.5, Description = 'dd')));
        a.put('P02', err(new Opportunity(Name = 'ERR', StageName = 'Prospecting', CloseDate = Date.newInstance(2031, 12, 1))));
        a.put('P03', err(new Opportunity(Name = 'ERR', StageName = 'Prospecting', CloseDate = Date.newInstance(2031, 3, 9), Amount = 5)));
        Account p = new Account(Name = 'Par');
        insert p;
        Account c = new Account(Name = 'REL', ParentId = p.Id);
        insert c;
        c = [SELECT Description, Site FROM Account WHERE Id = :c.Id];
        a.put('P04', new Map<String, Object>{ 'descriptionIsParentId' => c.Description == String.valueOf(p.Id), 'descriptionLength' => c.Description == null ? null : c.Description.length(), 'site' => c.Site });
        Account n = new Account(Name = 'REL2');
        insert n;
        n = [SELECT Description, Site FROM Account WHERE Id = :n.Id];
        a.put('P05', new Map<String, Object>{ 'description' => n.Description, 'site' => n.Site });
        Integer k = 0;
        for (Integer d : new Integer[]{ 10, 365, 366 }) {
            k++;
            a.put('P0' + (5 + k), err(new Opportunity(Name = 'LIM', StageName = 'Prospecting', CloseDate = Date.today().addDays(d))));
        }
        Map<String, Id> rt = new Map<String, Id>();
        for (String rn : new String[]{ 'GapParA', 'GapParB', 'GapParC', 'GapChildX', 'GapChildY' }) {
            rt.put(rn, Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get(rn).getRecordTypeId());
        }
        Account pa = new Account(Name = 'RtParA', RecordTypeId = rt.get('GapParA'));
        Account pb = new Account(Name = 'RtParB', RecordTypeId = rt.get('GapParB'));
        Account pc = new Account(Name = 'RtParC', RecordTypeId = rt.get('GapParC'));
        insert new Account[]{ pa, pb, pc };
        a.put('P09', rtInsert(new Account(Name = 'RtK1', ParentId = pa.Id, RecordTypeId = rt.get('GapChildX'))));
        a.put('P10', rtInsert(new Account(Name = 'RtK2', ParentId = pc.Id, RecordTypeId = rt.get('GapChildY'))));
        a.put('P11', rtInsert(new Account(Name = 'RtK3', ParentId = pa.Id, RecordTypeId = rt.get('GapChildY'))));
        a.put('P12', rtInsert(new Account(Name = 'RtK4', ParentId = pb.Id, RecordTypeId = rt.get('GapChildX'))));
        a.put('P13', rtInsert(new Account(Name = 'RtK5', RecordTypeId = rt.get('GapChildX'))));
        return a;
    }
    static Map<String, Object> rtInsert(Account acc) {
        Map<String, Object> r = new Map<String, Object>();
        try { insert acc; r.put('exception', null); }
        catch (DmlException e) { r.put('exception', e.getDmlStatusCode(0)); r.put('message', e.getDmlMessage(0)); }
        return r;
    }
}
