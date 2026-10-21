@IsTest
private class CedarBooleanObservationTest {
    private static Map<String,Object> cell(Flow.Interview interview, String name) {
        Object raw = interview.getVariableValue(name);
        Map<String,Object> result = new Map<String,Object>{
            'value' => raw,
            'booleanOrNull' => raw == null || raw instanceof Boolean,
            'castError' => null
        };
        try {
            Boolean value = (Boolean) raw;
            result.put('castValue', value);
        } catch (Exception error) {
            result.put('castError', error.getTypeName());
        }
        return result;
    }
    private static Map<String,Object> observe(Map<String,Object> inputs) {
        Flow.Interview interview = Flow.Interview.createInterview('Cedar_Boolean_Values', inputs);
        interview.start();
        Map<String,Object> result = new Map<String,Object>();
        for (String name : new List<String>{'defaultTrue','defaultFalse','defaultNull','assignedTrue','assignedFalse'}) {
            result.put(name, cell(interview, name));
        }
        return result;
    }
    @IsTest static void observeBooleans() {
        Map<String,Object> result = new Map<String,Object>{
            'omitted' => observe(new Map<String,Object>()),
            'overrideFalse' => observe(new Map<String,Object>{'defaultTrue' => false}),
            'overrideNull' => observe(new Map<String,Object>{'defaultTrue' => null})
        };
        System.assert(false, 'OBSERVED|booleanDefaults|' + JSON.serialize(result));
    }
}
