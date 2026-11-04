public class NullNumActionProbe {
    public static List<String> cases() { List<String> ids = new List<String>(); for (Integer i = 1; i <= 30; i++) ids.add('N' + String.valueOf(i).leftPad(2, '0')); return ids; }
    public static String observe() {
        Map<String, Object> results = new Map<String, Object>();
        for (String caseId : cases()) {
            Map<String, Object> inputs = new Map<String, Object>{'CaseId' => caseId, 'InputRecord' => new Account()};
            Invocable.Action action = Invocable.Action.createCustomAction('flow', 'NullNumAuto');
            action.setInvocations(new List<Map<String, Object>>{inputs});
            Invocable.Action.Result result = action.invoke()[0];
            System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
            results.put(caseId, result.getOutputParameters().get('Outcome'));
        }
        return JSON.serialize(results);
    }
}
