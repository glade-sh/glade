@IsTest
private class LookupFilterObservationTest {
    @IsTest
    static void observeInsertOutcomes() {
        List<Object> rows = new List<Object>();
        observe(rows, 'L01', 'RequiredTarget__c', 'Allowed', null, null, null, false, true);
        observe(rows, 'L02', 'RequiredTarget__c', 'Blocked', null, null, null, false, true);
        observe(rows, 'L03', 'OptionalTarget__c', 'Blocked', null, null, null, false, true);
        observe(rows, 'L04', null, null, null, null, null, false, false);
        observe(rows, 'L05', 'RequiredTarget__c', null, null, null, null, false, true);
        observe(rows, 'L06', 'SourceTarget__c', null, Date.newInstance(2026, 1, 1), Date.newInstance(2026, 1, 1), null, false, true);
        observe(rows, 'L07', 'SourceTarget__c', null, Date.newInstance(2026, 2, 1), Date.newInstance(2026, 1, 1), null, false, true);
        observe(rows, 'L08', 'SourceTarget__c', null, null, null, null, false, true);
        observe(rows, 'L09', 'FormulaTarget__c', null, null, null, 'Ready', false, true);
        observe(rows, 'L10', 'FormulaTarget__c', null, null, null, 'Blocked', false, true);
        observe(rows, 'L11', 'DirectTarget__c', null, null, null, 'Ready', false, true);
        observe(rows, 'L12', 'ValidationTarget__c', 'Allowed', null, null, null, true, true);
        observe(rows, 'L13', 'ValidationTarget__c', 'Blocked', null, null, null, true, true);
        observeText(rows, 'L14', 'TextEqualsFieldTarget__c', null, null);
        observeText(rows, 'L15', 'TextNotEqualFieldTarget__c', null, null);
        observeText(rows, 'L16', 'TextEqualsBlankTarget__c', null, null);
        observeText(rows, 'L17', 'TextNotEqualBlankTarget__c', null, null);
        observeText(rows, 'L18', 'DateNotEqualTarget__c', null, null);
        observeText(rows, 'L19', 'TextEqualsFieldTarget__c', '', '');
        observeText(rows, 'L20', 'TextNotEqualFieldTarget__c', '', '');
        observeText(rows, 'L21', 'TextEqualsBlankTarget__c', '', '');
        observeText(rows, 'L22', 'TextNotEqualBlankTarget__c', '', '');
        observeText(rows, 'L23', 'TextEqualsFieldTarget__c', 'match', 'match');
        observeText(rows, 'L24', 'TextNotEqualFieldTarget__c', 'left', 'right');
        observeText(rows, 'L25', 'TextEqualsBlankTarget__c', 'value', '');
        observeText(rows, 'L26', 'TextNotEqualBlankTarget__c', 'value', '');
        System.assert(false, 'LOOKUP_FILTER_OBSERVATION|' + JSON.serialize(rows));
    }

    private static void observe(List<Object> rows, String id, String lookupField, String category,
                                Date targetDate, Date sourceDate, String relatedState,
                                Boolean reject, Boolean createParent) {
        LookupFilterParent__c parent;
        if (createParent) {
            LookupFilterDetail__c detail;
            if (relatedState != null) {
                detail = new LookupFilterDetail__c(Name = 'detail ' + id, State__c = relatedState);
                insert detail;
            }
            parent = new LookupFilterParent__c(Name = 'parent ' + id, Category__c = category, RegionDate__c = targetDate);
            if (detail != null) parent.Related__c = detail.Id;
            insert parent;
        }

        LookupFilterChild__c child = new LookupFilterChild__c(Name = 'source ' + id, Reject__c = reject);
        if (lookupField != null) child.put(lookupField, parent.Id);
        if (sourceDate != null) child.SourceDate__c = sourceDate;
        capture(rows, id, child);
    }

    private static void observeText(List<Object> rows, String id, String lookupField, String targetText, String compareText) {
        LookupFilterParent__c parent = new LookupFilterParent__c(
            Name = 'parent ' + id, RegionText__c = targetText, CompareText__c = compareText);
        insert parent;

        LookupFilterBlankChild__c child = new LookupFilterBlankChild__c(Name = 'source ' + id);
        child.put(lookupField, parent.Id);
        capture(rows, id, child);
    }

    private static void capture(List<Object> rows, String id, SObject child) {
        try {
            Database.SaveResult result = Database.insert(child, false);
            if (result.isSuccess()) {
                rows.add(new Map<String, Object>{'id' => id, 'success' => true});
            } else {
                Database.Error error = result.getErrors()[0];
                rows.add(new Map<String, Object>{
                    'id' => id,
                    'success' => false,
                    'status' => String.valueOf(error.getStatusCode()),
                    'fields' => error.getFields(),
                    'message' => error.getMessage()
                });
            }
        } catch (Exception error) {
            rows.add(new Map<String, Object>{
                'id' => id,
                'success' => false,
                'status' => error.getTypeName(),
                'message' => error.getMessage()
            });
        }
    }
}
