@IsTest
private class SpruceRecordObservationTest {
    private static Map<String,Object> observe(String label) {
        Contact doomed = new Contact(LastName='Spruce deletion target');
        Database.insert(doomed, AccessLevel.SYSTEM_MODE);
        Flow.Interview interview = Flow.Interview.createInterview('Spruce_Explicit_Records', new Map<String,Object>{'label' => label, 'deleteId' => String.valueOf(doomed.Id)});
        interview.start();
        SObject selected = (SObject) interview.getVariableValue('foundRecord');
        List<SObject> selectedRows = (List<SObject>) interview.getVariableValue('foundRows');
        List<Object> rows = new List<Object>();
        if (selectedRows != null) {
            for (SObject row : selectedRows) {
                rows.add(new Map<String,Object>{'idPresent'=>row.Id != null, 'lastName'=>row.get('LastName'), 'title'=>row.get('Title')});
            }
        }
        Integer remaining = [SELECT count() FROM Contact WHERE Id=:doomed.Id];
        return new Map<String,Object>{
            'recordNull' => selected == null,
            'recordIdPresent' => selected != null && selected.Id != null,
            'recordLastName' => selected == null ? null : selected.get('LastName'),
            'recordTitle' => selected == null ? null : selected.get('Title'),
            'collectionNull' => selectedRows == null,
            'collectionRows' => rows,
            'capturedTitle' => interview.getVariableValue('capturedTitle'),
            'deleted' => remaining == 0
        };
    }
    @IsTest static void observeRecords() {
        Database.insert(new List<Contact>{new Contact(LastName='Spruce match',Title='Guide'),new Contact(LastName='Spruce match',Title='Guide')}, AccessLevel.SYSTEM_MODE);
        Map<String,Object> result = new Map<String,Object>{'matched'=>observe('Spruce match'),'missing'=>observe('Spruce missing')};
        System.assert(false, 'OBSERVED|explicitRecords|' + JSON.serialize(result));
    }
}
