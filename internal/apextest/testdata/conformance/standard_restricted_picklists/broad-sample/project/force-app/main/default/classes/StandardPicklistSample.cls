public class StandardPicklistSample {
    private static Id accountId;
    private static Id opportunityId;
    public static Map<String,Object> observe() {
        Account account = new Account(Name='Sample parent'); insert account; accountId = account.Id;
        Opportunity opportunity = new Opportunity(Name='Sample parent',StageName='Prospecting',CloseDate=Date.today());
        insert opportunity; opportunityId = opportunity.Id;
        Map<String,Object> observations = new Map<String,Object>();
        List<String> objects = new List<String>{'Opportunity','Case','Task','Task','Task','Event','Event','Contract','Contract','Contract','Order','Order','Quote','ContentVersion','ContentVersion'};
        List<String> fields = new List<String>{'ForecastCategoryName','Status','RecurrenceType','RecurrenceInstance','RecurrenceMonthOfYear','ShowAs','RecurrenceType','OwnerExpirationNotice','Status','BillingGeocodeAccuracy','Status','BillingGeocodeAccuracy','BillingGeocodeAccuracy','SharingOption','SharingPrivacy'};
        List<String> valid = new List<String>{'Pipeline','New',null,null,null,'Busy',null,'30','Draft','Address','Draft','Address','Address','A','N'};
        for (Integer i=0; i<objects.size(); i++) {
            String key=objects[i]+'.'+fields[i];
            Schema.SObjectType token=Schema.getGlobalDescribe().get(objects[i]);
            if (token==null || !token.getDescribe().fields.getMap().containsKey(fields[i])) { observations.put(key,'unavailable'); continue; }
            observations.put(key+'.restricted',token.getDescribe().fields.getMap().get(fields[i]).getDescribe().isRestrictedPicklist());
            SObject bad=fresh(objects[i]); bad.put(fields[i],'NotAnAllowedValue');
            observations.put(key+'.insert',summarize(Database.insert(bad,false), fields[i], token.getDescribe().fields.getMap().get(fields[i]).getDescribe().getLabel()));
            SObject row=fresh(objects[i]); row.put(fields[i],valid[i]);
            Database.SaveResult seed=Database.insert(row,false);
            observations.put(key+'.valid',summarize(seed, fields[i], ''));
            if(seed.isSuccess()) { SObject patch = token.newSObject(); patch.put('Id',row.get('Id')); patch.put(fields[i],'NotAnAllowedValue'); observations.put(key+'.update',summarize(Database.update(patch,false), fields[i], token.getDescribe().fields.getMap().get(fields[i]).getDescribe().getLabel())); }
        }
        return observations;
    }
    private static SObject fresh(String objectName) {
        if(objectName=='Opportunity') return new Opportunity(Name='Sample choice',StageName='Prospecting',CloseDate=Date.today());
        if(objectName=='Case') return new Case(Subject='Sample choice',Status='New',Origin='Web');
        if(objectName=='Task') return new Task(Subject='Sample choice');
        if(objectName=='Event') return new Event(Subject='Sample choice',StartDateTime=System.now(),EndDateTime=System.now().addHours(1));
        if(objectName=='Contract') return new Contract(AccountId=accountId,StartDate=Date.today(),ContractTerm=12,Status='Draft');
        if(objectName=='Order') return new Order(AccountId=accountId,EffectiveDate=Date.today(),Status='Draft');
        if(objectName=='Quote') { SObject row=Schema.getGlobalDescribe().get('Quote').newSObject(); row.put('Name','Sample choice'); row.put('OpportunityId',opportunityId); return row; }
        if(objectName=='ContentVersion') return new ContentVersion(Title='Sample choice',PathOnClient='sample.txt',VersionData=Blob.valueOf('Owned sample'));
        return null;
    }
    private static Object summarize(Database.SaveResult result, String fieldName, String label) {
        if (result.isSuccess()) return new List<Object>{true};
        Database.Error problem = result.getErrors()[0];
        Boolean matches = result.getErrors().size()==1 && String.valueOf(problem.getStatusCode())=='INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST' && problem.getMessage()==label+': bad value for restricted picklist field: NotAnAllowedValue' && problem.getFields()==new List<String>{fieldName};
        if (matches) return new List<Object>{false, true};
        return new List<Object>{false, String.valueOf(problem.getStatusCode()), problem.getMessage()};
    }
}
