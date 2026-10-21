public class OwnedFlowHandler {
    public class MissingOutputException extends Exception {}
    public static String stage = 'disabled';
    public static String flowName = 'OwnedFlowName';
    public static String inputName = 'record';
    public static Boolean cloneInput = false;
    public static Integer calls = 0;
    public static String returnedName;
    public static String returnedDescription;
    public static Boolean returnedSuccess;
    public static String returnedErrorCode;
    public static String returnedErrorMessage;

    public static Invocable.Action.Result invoke(Account inputRecord) {
        Invocable.Action action = Invocable.Action.createCustomAction('flow', flowName);
        action.setInvocationParameter(inputName, inputRecord);
        if (flowName == 'OwnedFlowFault') {
            action.setInvocationParameter('runFault', true);
        }
        List<Invocable.Action.Result> results = action.invoke();
        System.assertEquals(1, results.size());
        return results[0];
    }

    public static Account outputRecord(Invocable.Action.Result result) {
        Account output = (Account)result.getOutputParameters().get('record');
        if (output == null) {
            throw new MissingOutputException('Synthetic handler requires output record');
        }
        return output;
    }

    public static void handle(List<Account> records, Boolean writeBack) {
        for (Account original : records) {
            calls++;
            Account input = original;
            if (cloneInput) {
                input = (Account)original.clone(true, true, false, false);
            }
            Invocable.Action.Result result = invoke(input);
            returnedSuccess = result.isSuccess();
            if (!returnedSuccess) {
                System.assertEquals(1, result.getErrors().size());
                returnedErrorCode = result.getErrors()[0].getCode();
                returnedErrorMessage = result.getErrors()[0].getMessage();
                original.addError(returnedErrorMessage);
                continue;
            }
            System.assertEquals(0, result.getErrors().size());
            Account output = outputRecord(result);
            System.assertEquals(original.Id, output.Id);
            returnedName = output.Name;
            returnedDescription = output.Description;
            // The caller performs the write-back; the Flow returns a clone.
            // After/delete paths observe outputs without mutating read-only rows.
            if (writeBack) {
                original.Name = output.Name;
                original.Description = output.Description;
            }
        }
    }
}
