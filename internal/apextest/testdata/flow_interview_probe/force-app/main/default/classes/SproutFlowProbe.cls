public class SproutFlowProbe {
    public static Map<String,Object> observe(String row, Boolean subclass) {
        Map<String,Object> observation = new Map<String,Object>{'row'=>row,'subclass'=>subclass};
        Account original = new Account(Name='Seed', Description='Keep');
        Account prior = new Account(Name='Prior', Description='Earlier');
        Map<String,Object> inputs = new Map<String,Object>{'record'=>original,'recordPrior'=>prior};
        String flowName = 'SproutRecord';
        if (row == 'FI04' || row == 'FI05' || row == 'FI06') { flowName = 'SproutNull'; }
        if (row == 'FI07' || row == 'FI08' || row == 'FI09') {
            flowName = 'SproutError'; inputs.put('failMode', row == 'FI09');
        }
        if (row == 'FI10') { inputs.remove('record'); }
        if (row == 'FI11') { inputs.put('record', null); }
        if (row == 'FI12') { flowName = 'SproutInputOnly'; }
        if (row == 'FI13') { inputs.put('absent', 42); }
        if (row == 'FI14' || row == 'FI15' || row == 'FI16') {
            flowName = 'SproutScalar'; inputs = new Map<String,Object>{'seed'=>'leaf'};
            if (row == 'FI15') { inputs.put('seed', 42); }
        }
        observation.put('flowName', flowName);
        Flow.Interview interview;
        try {
            if (!subclass) { interview = Flow.Interview.createInterview(flowName, inputs); }
            else if (flowName == 'SproutRecord') { interview = new Flow.Interview.SproutRecord(inputs); }
            else if (flowName == 'SproutNull') { interview = new Flow.Interview.SproutNull(inputs); }
            else if (flowName == 'SproutError') { interview = new Flow.Interview.SproutError(inputs); }
            else if (flowName == 'SproutInputOnly') { interview = new Flow.Interview.SproutInputOnly(inputs); }
            else { interview = new Flow.Interview.SproutScalar(inputs); }
            interview.start();
            observation.put('started', true);
            observation.put('scalarOutput', interview.getVariableValue('result'));
            observation.put('absentOutput', interview.getVariableValue('absent'));
            observation.put('priorNameOutput', interview.getVariableValue('priorName'));
            Account output = (Account)interview.getVariableValue('record');
            snapshot(observation, 'output', output);
            snapshot(observation, 'inputAfterStart', original);
            snapshot(observation, 'priorAfterStart', prior);
            Account current = original;
            if (row == 'FI02' || row == 'FI05') { current = output; }
            if ((row == 'FI03' || row == 'FI06') && output != null) {
                for (String field : output.getPopulatedFieldsAsMap().keySet()) { current.put(field, output.get(field)); }
            }
            if (row == 'FI08' && output != null) {
                for (Database.Error error : output.getErrors()) { current.addError(error.getMessage()); }
            }
            snapshot(observation, 'afterCallerWriteBack', current);
            if (row == 'FI01' && output != null) {
                output.Name = 'AliasCheck';
                observation.put('inputNameAfterOutputMutation', original.Name);
                observation.put('reReadName', ((Account)interview.getVariableValue('record')).Name);
            }
        } catch (Exception error) {
            observation.put('exceptionType', error.getTypeName());
            observation.put('exceptionMessage', error.getMessage());
            snapshot(observation, 'inputAfterException', original);
            snapshot(observation, 'priorAfterException', prior);
        }
        return observation;
    }
    private static void snapshot(Map<String,Object> observation, String prefix, Account record) {
        observation.put(prefix+'IsNull', record == null);
        if (record == null) { return; }
        observation.put(prefix+'Name', record.Name);
        observation.put(prefix+'Description', record.Description);
        observation.put(prefix+'DescriptionPopulated', record.getPopulatedFieldsAsMap().containsKey('Description'));
        observation.put(prefix+'HasErrors', record.hasErrors());
        List<String> errors = new List<String>();
        for (Database.Error error : record.getErrors()) { errors.add(String.valueOf(error.getStatusCode())+'|'+error.getMessage()); }
        observation.put(prefix+'Errors', errors);
    }
}
