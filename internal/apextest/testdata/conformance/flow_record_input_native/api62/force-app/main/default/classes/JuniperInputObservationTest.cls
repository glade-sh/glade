@IsTest
private class JuniperInputObservationTest {
    private static Map<String,Object> observe(String accountName) {
        Flow.Interview interview = Flow.Interview.createInterview('Juniper_Record_Input', new Map<String,Object>{'accountName'=>accountName,'parentName'=>accountName == 'Juniper funds' ? 'Juniper existing parent' : 'Juniper absent parent'});
        interview.start();
        SObject parent = (SObject)interview.getVariableValue('parentRecord');
        Id parentId = parent == null ? null : parent.Id;
        String childId = (String)interview.getVariableValue('childId');
        Contact child = [SELECT AccountId FROM Contact WHERE Id=:childId];
        List<Account> linkedParents = [SELECT Name FROM Account WHERE Id=:child.AccountId];
        String parentType = null;
        String parentTypeError = null;
        try { if (parent != null) parentType = parent.getSObjectType().getDescribe().getName(); }
        catch (Exception error) { parentTypeError = error.getTypeName(); }
        return new Map<String,Object>{
            'balance'=>interview.getVariableValue('balance'),
            'valid'=>interview.getVariableValue('valid'),
            'parentIdPresent'=>parentId != null,
            'parentType'=>parentType,
            'parentTypeError'=>parentTypeError,
            'childIdPresent'=>String.isNotBlank(childId),
            'childLinked'=>parentId != null && child.AccountId == parentId,
            'childAccountIdPresent'=>child.AccountId != null,
            'linkedAccountName'=>linkedParents.size() == 1 ? linkedParents[0].Name : null
        };
    }
    private static Map<String,Object> textBranch(String token) {
        Flow.Interview interview = Flow.Interview.createInterview('Juniper_Text_Branch',new Map<String,Object>{'token'=>token});
        interview.start();
        List<String> labels = (List<String>)interview.getVariableValue('labels');
        return new Map<String,Object>{'count'=>labels == null ? -1 : labels.size(),'title'=>interview.getVariableValue('title')};
    }
    @IsTest static void observeInputs() {
        Database.insert(new List<Account>{new Account(Name='Juniper funds',AnnualRevenue=17.25),new Account(Name='Juniper zero',AnnualRevenue=0),new Account(Name='Juniper existing parent')},AccessLevel.SYSTEM_MODE);
        Map<String,Object> result = new Map<String,Object>{
            'decimal'=>observe('Juniper funds'),
            'zero'=>observe('Juniper zero'),
            'missing'=>observe('Juniper absent'),
            'textMatch'=>textBranch('Guide'),
            'textMiss'=>textBranch('other')
        };
        System.assert(false, 'OBSERVED|observeInputs|' + JSON.serialize(result));
    }
}
