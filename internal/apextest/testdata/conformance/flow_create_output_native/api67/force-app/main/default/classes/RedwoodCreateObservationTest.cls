@IsTest
private class RedwoodCreateObservationTest {
    @IsTest static void observeCreatedId() {
        Flow.Interview interview = Flow.Interview.createInterview('Redwood_Create_Output', new Map<String,Object>{'revenue' => 17.25});
        interview.start();
        Object raw = interview.getVariableValue('createdId');
        String createdId = null;
        String castError = null;
        try { createdId = (String) raw; } catch (Exception error) { castError = error.getTypeName(); }
        Integer recordCount = [SELECT count() FROM Account WHERE Name='Redwood inserted row'];
        Integer matchingCount = [SELECT count() FROM Account WHERE Id=:createdId AND Name='Redwood inserted row'];
        Account scalarRow = [SELECT AnnualRevenue FROM Account WHERE Name='Redwood inserted row'];
        Account literalRow = [SELECT AnnualRevenue FROM Account WHERE Name='Redwood literal control'];
        Map<String,Object> result = new Map<String,Object>{
            'idPresent' => String.isNotBlank(createdId),
            'recordCount' => recordCount,
            'matchingCount' => matchingCount,
            'castError' => castError,
            'scalarRevenue' => scalarRow.AnnualRevenue,
            'literalRevenue' => literalRow.AnnualRevenue
        };
        System.assert(false, 'OBSERVED|flowCreateOutput|' + JSON.serialize(result));
    }
}
