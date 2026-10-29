public class LookupFilterListProbe {
    public static Map<String,Object> observe() {
        List<FilterTarget__c> targets = new List<FilterTarget__c>{
            new FilterTarget__c(Name='t1',Kind__c='Alpha',Tag__c='red'),
            new FilterTarget__c(Name='t2',Kind__c='Beta',Tag__c='blue'),
            new FilterTarget__c(Name='t3',Kind__c='Gamma',Tag__c='green'),
            new FilterTarget__c(Name='t4',Kind__c='Alpha',Tag__c='red,blue'),
            new FilterTarget__c(Name='t5',Kind__c='Alpha',Tag__c='RED'),
            new FilterTarget__c(Name='t6',Kind__c='Alpha',Tag__c=' red'),
            new FilterTarget__c(Name='t7',Kind__c=null,Tag__c=null),
            new FilterTarget__c(Name='t8',Kind__c='alpha',Tag__c='Blue')
        };
        Database.insert(targets, true, AccessLevel.SYSTEM_MODE);
        Map<String,Object> observed = new Map<String,Object>();
        for (String holder : new List<String>{'PEq','PEqT','PNe','TEq','TNe','TEqWs','TEqCase'}) {
            for (FilterTarget__c target : targets) {
                SObject row = Schema.getGlobalDescribe().get('Holder' + holder + '__c').newSObject();
                row.put('Pick__c', target.Id);
                Database.SaveResult result = Database.insert(row, false, AccessLevel.SYSTEM_MODE);
                observed.put(holder + '|' + target.Name, result.isSuccess() ? 'ok' : String.valueOf(result.getErrors()[0].getStatusCode()) + '|' + result.getErrors()[0].getMessage());
            }
        }
        return observed;
    }
}
