public class FixedSearchHelper {
    public static String fmtGroups(List<List<SObject>> g){
        String m='';
        for(List<SObject> rows:g){
            List<String> l=new List<String>();
            for(SObject r:rows) l.add((String)r.get('Department'));
            m+='['+String.join(l,',')+']';
        }
        return m;
    }
    public static String obs(String q){
        try{ return fmtGroups(Search.query(q)); }catch(Exception e){ return e.getTypeName()+'|'+e.getMessage(); }
    }
    public static String obsInline(){
        try{ return fmtGroups([FIND 'Fixedasync' IN ALL FIELDS RETURNING Contact(Id,Department ORDER BY Department)]); }catch(Exception e){ return e.getTypeName()+'|'+e.getMessage(); }
    }
    public static String work(String mode,String q,String extra){
        String r='';
        if(mode=='inline') r=obsInline();
        else if(mode=='setF'){
            try{ Test.setFixedSearchResults(new List<Id>{(Id)extra}); r='set-ok|'+obs(q); }catch(Exception e){ r='set-ERR '+e.getTypeName()+'|'+e.getMessage(); }
        } else r=obs(q);
        return r+'@sosl='+Limits.getSoslQueries()+'@isTest='+Test.isRunningTest();
    }
    @future public static void fut(Id t,String mode,String q,String extra){
        update new Account(Id=t,Description=work(mode,q,extra));
    }
    public static List<Contact> seed12(){
        List<Contact> cs=new List<Contact>{
            new Contact(LastName='Goldfox Amberquartz Beta runners',Department='A',Email='aria.quartz@example.invalid',Phone='+1 (415) 555-1287'),
            new Contact(LastName='Goldfox Amberquartz Betta running',Department='B',Email='river.cloud@example.invalid',Phone='650-555-1288'),
            new Contact(LastName='Goldfox Bta Amberquartz runner',Department='C',Email='arya@example.invalid'),
            new Contact(LastName='Goldfox AlphaBetamax',Department='D',Email='symbol@example.invalid'),
            new Contact(LastName='Goldfox Beta Amberquartz run',Department='E'),
            new Contact(LastName='Goldfox prune',Department='F'),
            new Contact(LastName='Goldfox Cedar runs',Department='G'),
            new Contact(LastName='Goldfox Anne-Marie Oak',Department='H'),
            new Contact(LastName='Goldfox O\'Brien Elm',Department='I'),
            new Contact(LastName='Goldfox Pine',Department='J',Phone='(415) 555-0199'),
            new Contact(LastName='Goldfox Fir',Department='K',Phone='415-555-0188'),
            new Contact(LastName='Goldfox Ash',Department='L',Phone='4155550177')};
        insert cs;
        return cs;
    }
    public static String termObs(String q){
        try{
            List<String> l=new List<String>();
            for(SObject r:Search.query(q)[0]) l.add((String)r.get('Department'));
            return String.join(l,',');
        }catch(Exception e){ return e.getTypeName()+'|'+e.getMessage(); }
    }
}
