@IsTest
private class HazelRoutingObservationTest {
    private static List<String> names(Object raw) {
        List<String> result = new List<String>();
        if (raw != null) for (SObject record : (List<SObject>)raw) result.add((String)record.get('Name'));
        result.sort();
        return result;
    }
    private static Map<String,Object> observe(String text) {
        Flow.Interview interview = Flow.Interview.createInterview('Hazel_Record_Routing', new Map<String,Object>{'text'=>text,'place'=>'East','ceiling'=>17.25});
        interview.start();
        SObject selected = (SObject)interview.getVariableValue('selectedRecord');
        SObject created = (SObject)interview.getVariableValue('createdRecord');
        Id createdId = created == null ? null : created.Id;
        Integer createdMatches = [SELECT count() FROM Account WHERE Id=:createdId AND Name='Hazel spawned'];
        return new Map<String,Object>{
            'orNames'=>names(interview.getVariableValue('orRecords')),
            'lowNames'=>names(interview.getVariableValue('lowRecords')),
            'selectedName'=>selected == null ? null : selected.get('Name'),
            'selectedPhone'=>selected == null ? null : selected.get('Phone'),
            'createdIdPresent'=>createdId != null,
            'createdRevenue'=>created == null ? null : created.get('AnnualRevenue'),
            'storedRevenue'=>createdId == null ? null : [SELECT AnnualRevenue FROM Account WHERE Id=:createdId].AnnualRevenue,
            'createdMatches'=>createdMatches,
            'caseResult'=>interview.getVariableValue('caseResult')
        };
    }
    @IsTest static void observeRouting() {
        Database.insert(new List<Account>{
            new Account(Name='Hazel eastbill',BillingCity='East',AnnualRevenue=17.25,Phone='123'),
            new Account(Name='Hazel eastship',ShippingCity='East',AnnualRevenue=23.5,Phone='456'),
            new Account(Name='Hazel blank')
        }, AccessLevel.SYSTEM_MODE);
        Map<String,Object> result = new Map<String,Object>();
        for (String text : new List<String>{'guide','Guide','GUIDE'}) result.put(text,observe(text));
        System.assert(false, 'OBSERVED|recordRouting|' + JSON.serialize(result));
    }
}
