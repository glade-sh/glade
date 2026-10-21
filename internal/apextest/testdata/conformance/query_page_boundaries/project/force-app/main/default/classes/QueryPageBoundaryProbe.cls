public class QueryPageBoundaryProbe {
    public static Map<String,Object> observe() {
        Account row = new Account(Name='Meadow numeric observation', NumberOfEmployees=1);
        insert row;
        Integer one = 1;
        Map<String,Object> observations = new Map<String,Object>();
        observations.put('numericLiteralBind', [SELECT Id FROM Account WHERE NumberOfEmployees = :1 AND Id = :row.Id].size());
        observations.put('numericVariableBind', [SELECT Id FROM Account WHERE NumberOfEmployees = :one AND Id = :row.Id].size());
        observations.put('numericLiteralControl', [SELECT Id FROM Account WHERE NumberOfEmployees = 1 AND Id = :row.Id].size());
        observations.put('apexLogDeletable', ApexLog.SObjectType.getDescribe().isDeletable());
        observations.put('contactDeletableControl', Contact.SObjectType.getDescribe().isDeletable());
        observations.put('pageTokenUrl', Page.MeadowMixed.getUrl());
        observations.put('explicitPageUrl', new PageReference('/apex/MeadowMixed').getUrl());
        return observations;
    }
}
