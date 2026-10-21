public class UpdateOptionsProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        Database.DMLOptions options = new Database.DMLOptions();
        options.OptAllOrNone = true;
        try {
            Database.update(new List<SObject>{new Account()}, options, AccessLevel.SYSTEM_MODE);
            result.put('optionsTrue', 'accepted');
        } catch(Exception issue) {
            result.put('optionsTrueType', issue.getTypeName());
            result.put('optionsTrueMessage', issue.getMessage());
        }
        options.OptAllOrNone = false;
        try {
            List<Database.SaveResult> outcomes = Database.update(new List<SObject>{new Account()}, options, AccessLevel.SYSTEM_MODE);
            result.put('optionsFalseCount', outcomes.size());
            result.put('optionsFalseSuccess', outcomes[0].isSuccess());
            result.put('optionsFalseStatus', String.valueOf(outcomes[0].getErrors()[0].getStatusCode()));
            result.put('optionsFalseMessage', outcomes[0].getErrors()[0].getMessage());
        } catch(Exception issue) {
            result.put('optionsFalseType', issue.getTypeName());
            result.put('optionsFalseMessage', issue.getMessage());
        }
        try {
            Account record = new Account();
            update record;
            result.put('statement', 'accepted');
        } catch(Exception issue) {
            result.put('statementType', issue.getTypeName());
            result.put('statementMessage', issue.getMessage());
        }
        try {
            Database.update(new List<SObject>{new Account()});
            result.put('bare', 'accepted');
        } catch(Exception issue) {
            result.put('bareType', issue.getTypeName());
            result.put('bareMessage', issue.getMessage());
        }
        return result;
    }
}
