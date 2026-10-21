public class EventForwardingProbe {
    private static Database.SaveResult forwardOne(SObject value) {
        return EventBus.publish(value);
    }
    private static List<Database.SaveResult> forwardMany(List<SObject> values) {
        return EventBus.publish(values);
    }
    private static Map<String,Object> describe(Database.SaveResult result) {
        List<String> codes = new List<String>();
        for (Database.Error problem : result.getErrors()) {
            codes.add(String.valueOf(problem.getStatusCode()));
        }
        return new Map<String,Object>{'success'=>result.isSuccess(), 'idNull'=>result.getId()==null, 'codes'=>codes};
    }
    private static Map<String,Object> failure(Exception problem) {
        return new Map<String,Object>{'type'=>problem.getTypeName(), 'message'=>problem.getMessage()};
    }
    public static Map<String,Object> observe() {
        Map<String,Object> rows = new Map<String,Object>();
        try {
            rows.put('directConcrete', describe(EventBus.publish(new BatchApexErrorEvent())));
        } catch (Exception problem) { rows.put('directConcrete', failure(problem)); }
        try {
            SObject erased = new BatchApexErrorEvent();
            rows.put('erasedConcrete', describe(EventBus.publish(erased)));
        } catch (Exception problem) { rows.put('erasedConcrete', failure(problem)); }
        try {
            rows.put('forwardConcrete', describe(forwardOne(new BatchApexErrorEvent())));
        } catch (Exception problem) { rows.put('forwardConcrete', failure(problem)); }
        try {
            SObject erased = Schema.getGlobalDescribe().get('BatchApexErrorEvent').newSObject();
            rows.put('erasedReflective', describe(EventBus.publish(erased)));
        } catch (Exception problem) { rows.put('erasedReflective', failure(problem)); }
        try {
            rows.put('forwardReflective', describe(forwardOne(Schema.getGlobalDescribe().get('BatchApexErrorEvent').newSObject())));
        } catch (Exception problem) { rows.put('forwardReflective', failure(problem)); }
        try {
            rows.put('directErasedList', describe(EventBus.publish(new List<SObject>{new BatchApexErrorEvent()})[0]));
        } catch (Exception problem) { rows.put('directErasedList', failure(problem)); }
        try {
            rows.put('forwardErasedList', describe(forwardMany(new List<SObject>{new BatchApexErrorEvent()})[0]));
        } catch (Exception problem) { rows.put('forwardErasedList', failure(problem)); }
        return rows;
    }
}
