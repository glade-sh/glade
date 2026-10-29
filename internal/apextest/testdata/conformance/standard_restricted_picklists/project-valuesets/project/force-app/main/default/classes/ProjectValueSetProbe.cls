public class ProjectValueSetProbe {
    public static Map<String,Object> observe() {
        Account account=new Account(Name='Value set parent'); insert account;
        Map<String,Object> observations=new Map<String,Object>();
        List<String> objects=new List<String>{'Contract','Order','Task'};
        List<String> declared=new List<String>{'CustomStatus','CustomOrder','CustomTask'};
        List<String> omitted=new List<String>{'Draft','Draft','Not Started'};
        for(Integer i=0;i<objects.size();i++) {
            String objectName=objects[i];
            Schema.SObjectType token=Schema.getGlobalDescribe().get(objectName);
            Schema.DescribeFieldResult description=token.getDescribe().fields.getMap().get('Status').getDescribe();
            observations.put(objectName+'.restricted',description.isRestrictedPicklist());
            for(String value:new List<String>{declared[i],omitted[i]}) {
                Boolean active=false; Boolean present=false;
                for(Schema.PicklistEntry entry:description.getPicklistValues()) if(entry.getValue()==value) { present=true; if(entry.isActive()) active=true; }
                observations.put(objectName+'.'+value+'.active',active); observations.put(objectName+'.'+value+'.present',present);
            }
            for(String value:new List<String>{declared[i],omitted[i],'NotAnAllowedValue'}) {
                SObject row=fresh(objectName,account.Id); row.put('Status',value);
                observations.put(objectName+'.'+value+'.insert',summarize(Database.insert(row,false)));
                SObject seed=fresh(objectName,account.Id); seed.put('Status',declared[i]); insert seed;
                SObject patch=token.newSObject();patch.put('Id',seed.get('Id'));patch.put('Status',value);
                observations.put(objectName+'.'+value+'.update',summarize(Database.update(patch,false)));
            }
        }
        return observations;
    }
    private static SObject fresh(String name, Id parent) {
        if(name=='Contract') return new Contract(AccountId=parent,StartDate=Date.today(),ContractTerm=12);
        if(name=='Order') return new Order(AccountId=parent,EffectiveDate=Date.today());
        return new Task(Subject='Value set choice');
    }
    private static Object summarize(Database.SaveResult result) {
        if(result.isSuccess()) return new List<Object>{true};
        Database.Error problem=result.getErrors()[0];
        return new List<Object>{false,String.valueOf(problem.getStatusCode()),problem.getMessage()};
    }
}
