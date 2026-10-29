public class RestrictedPicklistProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> observations = new Map<String, Object>();
        List<String> objects = new List<String>{'Account','Account','Contact','Lead','Task'};
        List<String> fields = new List<String>{'BillingGeocodeAccuracy','ShippingGeocodeAccuracy','MailingGeocodeAccuracy','GeocodeAccuracy','CallType'};
        List<String> valid = new List<String>{'Address','Address','Address','Address','Inbound'};
        for (Integer i = 0; i < objects.size(); i++) {
            Schema.SObjectType token = Schema.getGlobalDescribe().get(objects[i]);
            Schema.DescribeFieldResult description = token.getDescribe().fields.getMap().get(fields[i]).getDescribe();
            observations.put(objects[i]+'.'+fields[i]+'.restricted', description.isRestrictedPicklist());
            for (String kind : new List<String>{'bad','valid','null'}) {
                SObject row = fresh(objects[i]);
                row.put(fields[i], kind == 'bad' ? 'NotAnAllowedValue' : (kind == 'valid' ? valid[i] : null));
                observations.put(objects[i]+'.'+fields[i]+'.'+kind, summarize(Database.insert(row, false)));
            }
            SObject row = fresh(objects[i]);
            row.put(fields[i],valid[i]);
            insert row;
            row.put(fields[i],'NotAnAllowedValue');
            observations.put(objects[i]+'.'+fields[i]+'.update',summarize(Database.update(row,false)));
        }
        observations.put('unrestricted',summarize(Database.insert(new Account(Name='Open choice',Industry='NotAnAllowedValue'),false)));
        observations.put('custom',summarize(Database.insert(new Account(Name='Custom choice',ProbeChoice__c='NotAnAllowedValue'),false)));
        try {
            insert new Account(Name='Statement choice',BillingGeocodeAccuracy='NotAnAllowedValue');
            observations.put('statement','accepted');
        } catch (DmlException problem) {
            observations.put('statement',problem.getDmlMessage(0));
        }
        return observations;
    }
    private static SObject fresh(String objectName) {
        if (objectName == 'Account') return new Account(Name='Restricted choice');
        if (objectName == 'Contact') return new Contact(LastName='Restricted choice');
        if (objectName == 'Lead') return new Lead(LastName='Restricted choice',Company='Synthetic');
        return new Task(Subject='Restricted choice');
    }
    private static Map<String,Object> summarize(Database.SaveResult result) {
        List<Object> errors = new List<Object>();
        for (Database.Error problem : result.getErrors()) {
            errors.add(new Map<String,Object>{'status'=>String.valueOf(problem.getStatusCode()),'message'=>problem.getMessage(),'fields'=>problem.getFields()});
        }
        return new Map<String,Object>{'success'=>result.isSuccess(),'errors'=>errors};
    }
}
