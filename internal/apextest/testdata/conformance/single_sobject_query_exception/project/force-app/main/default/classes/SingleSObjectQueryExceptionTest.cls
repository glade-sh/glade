@IsTest(IsParallel=true) private class SingleSObjectQueryExceptionTest {
    private static Map<String,Object> queryOne(String wantedName, String wantedKey) {
        Map<String,Object> observed = new Map<String,Object>();
        try {
            ProbeCredential__c credential = [
                SELECT Id, Name, ProbeKey__c, ProbeValue__c
                FROM ProbeCredential__c
                WHERE Name = :wantedName AND ProbeKey__c = :wantedKey
                LIMIT 1
            ];
            observed.put('outcome', 'row');
            observed.put('value', credential.ProbeValue__c);
        } catch (Exception ex) {
            observed.put('type', ex.getTypeName());
            observed.put('message', ex.getMessage());
            observed.put('line', ex.getLineNumber());
        }
        return observed;
    }

    private static void record(String id, Map<String,Object> observed) {
        System.assert(false, 'OBSERVED|' + id + '|' + JSON.serialize(observed));
    }

    private static Map<String,Object> exceptionObservation(Exception ex) {
        return new Map<String,Object>{'type'=>ex.getTypeName(), 'message'=>ex.getMessage(), 'line'=>ex.getLineNumber()};
    }

    private static void recordWrapped(String id, String wantedName, String wantedKey) {
        try {
            String value = ProbeSettingsService.getValue(wantedName, wantedKey);
            record(id, new Map<String,Object>{'outcome'=>'row', 'value'=>value});
        } catch (Exception ex) {
            record(id, new Map<String,Object>{'query'=>ProbeSettingsService.lastCaught, 'thrown'=>exceptionObservation(ex)});
        }
    }

    @IsTest static void noRowsAssignment() {
        record('noRowsAssignment', queryOne('absent', 'absent'));
    }

    @IsTest static void filteredMissAssignment() {
        insert new ProbeCredential__c(Name='known-name', ProbeKey__c='stored-key', ProbeValue__c='ignored');
        record('filteredMissAssignment', queryOne('known-name', 'wanted-key'));
    }

    @IsTest static void matchingAssignment() {
        insert new ProbeCredential__c(Name='known-name', ProbeKey__c='wanted-key', ProbeValue__c='selected-value');
        record('matchingAssignment', queryOne('known-name', 'wanted-key'));
    }

    @IsTest static void emptyListControl() {
        List<ProbeCredential__c> rows = [
            SELECT Id, Name, ProbeKey__c, ProbeValue__c
            FROM ProbeCredential__c
            WHERE Name = 'absent' AND ProbeKey__c = 'absent'
        ];
        record('emptyListControl', new Map<String,Object>{'outcome'=>'list', 'size'=>rows.size()});
    }

    @IsTest static void wrappedNoRows() {
        recordWrapped('wrappedNoRows', 'absent', 'absent');
    }

    @IsTest static void wrappedFilteredMiss() {
        insert new ProbeCredential__c(Name='known-name', ProbeKey__c='stored-key', ProbeValue__c='ignored');
        recordWrapped('wrappedFilteredMiss', 'known-name', 'wanted-key');
    }

    @IsTest static void wrappedMatching() {
        insert new ProbeCredential__c(Name='known-name', ProbeKey__c='wanted-key', ProbeValue__c='selected-value');
        recordWrapped('wrappedMatching', 'known-name', 'wanted-key');
    }
}
