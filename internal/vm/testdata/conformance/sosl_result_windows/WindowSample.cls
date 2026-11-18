public class WindowSample {
    public static void seed(){insert new List<Account>{new Account(Name='Copperwindow A'),new Account(Name='Copperwindow B'),new Account(Name='Copperwindow C'),new Account(Name='Copperwindow D'),new Account(Name='Copperwindow E')};
insert new List<Contact>{new Contact(LastName='Copperwindow K'),new Contact(LastName='Copperwindow L'),new Contact(LastName='Copperwindow M')};
insert new List<Lead>{new Lead(LastName='Copperwindow X',Company='Owned'),new Lead(LastName='Copperwindow Y',Company='Owned')};}
    public static Map<String,String> observe(Map<String,String> cases,Boolean repeatOrder) {
        Map<String,Id> byLabel=new Map<String,Id>();
        for(Account a:[SELECT Id,Name FROM Account WHERE Name LIKE 'Copperwindow %']) byLabel.put(a.Name.substring(13),a.Id);
        for(Contact c:[SELECT Id,LastName FROM Contact WHERE LastName LIKE 'Copperwindow %']) byLabel.put(c.LastName.substring(13),c.Id);
        for(Lead l:[SELECT Id,LastName FROM Lead WHERE LastName LIKE 'Copperwindow %']) byLabel.put(l.LastName.substring(13),l.Id);
        System.assertEquals(10,byLabel.size());
        if(Test.isRunningTest()){
            List<Id> ids=new List<Id>();
            for(String label:(repeatOrder ? 'B,M,Y,C,K,D,X,E,L,A' : 'L,D,X,B,K,E,M,Y,C,A').split(',')) ids.add(byLabel.get(label));
            Test.setFixedSearchResults(ids);
        }
        Map<Id,String> labels=new Map<Id,String>();
        for(String lbl:byLabel.keySet())labels.put(byLabel.get(lbl),lbl);
        Map<String,String> result=new Map<String,String>();
        for(String key:cases.keySet()){
            String marker='';
            try {
                List<List<SObject>> groups=Search.query(cases.get(key));
                for(List<SObject> rows:groups){
                    List<String> members=new List<String>();
                    for(SObject row:rows) members.add(labels.get((Id)row.get('Id')));
                    marker+='['+String.join(members,',')+']';
                }
            } catch(Exception ex){marker=ex.getTypeName()+'|'+ex.getMessage();}
            result.put(key,marker);
        }
        return result;
    }
}
