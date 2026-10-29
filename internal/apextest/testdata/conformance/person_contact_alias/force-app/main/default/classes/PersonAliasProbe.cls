public class PersonAliasProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> r = new Map<String,Object>();
        Map<String,Schema.SObjectField> f = Account.SObjectType.getDescribe().fields.getMap();
        r.put('textExists',f.containsKey('Drift_Tag__pc'));
        r.put('booleanExists',f.containsKey('Is_Ready__pc'));
        r.put('missingAbsent',!f.containsKey('Absent_Field__pc'));
        r.put('textType',String.valueOf(f.get('Drift_Tag__pc').getDescribe().getType()));
        r.put('textLength',f.get('Drift_Tag__pc').getDescribe().getLength());
        r.put('booleanType',String.valueOf(f.get('Is_Ready__pc').getDescribe().getType()));
        Account a = new Account(LastName='River',Drift_Tag__pc='alpha',Is_Ready__pc=true);
        r.put('constructed',a.Drift_Tag__pc+':'+a.Is_Ready__pc);
        insert a;
        a=[SELECT Id,Drift_Tag__pc,Is_Ready__pc FROM Account WHERE Id=:a.Id];
        r.put('queried',a.Drift_Tag__pc+':'+a.Is_Ready__pc);
        a.Drift_Tag__pc='beta'; a.Is_Ready__pc=false; update a;
        a=[SELECT Id,Drift_Tag__pc,Is_Ready__pc FROM Account WHERE Id=:a.Id];
        r.put('updated',a.Drift_Tag__pc+':'+a.Is_Ready__pc);
        a.Drift_Tag__pc=null; update a;
        a=[SELECT Id,Drift_Tag__pc FROM Account WHERE Id=:a.Id];
        r.put('cleared',a.Drift_Tag__pc==null);
        return r;
    }
}
