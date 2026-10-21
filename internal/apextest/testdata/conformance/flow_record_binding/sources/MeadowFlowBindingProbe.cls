public class MeadowFlowBindingProbe {
    public static Map<String, Boolean> observe() {
        Map<String, Boolean> answers = new Map<String, Boolean>();
        observeCase(answers, 1, 'scalar_with_id');
        observeCase(answers, 6, 'scalar_null');
        observeCase(answers, 11, 'scalar_missing_id');
        observeCase(answers, 16, 'collection_empty');
        observeCase(answers, 21, 'collection_with_id');
        observeTrigger(answers);
        return answers;
    }

    private static void observeCase(Map<String, Boolean> answers, Integer firstCell, String mode) {
        Savepoint point = Database.setSavepoint();
        try {
            Account target = new Account(Name='Meadow retained name', Description='Before');
            insert target;
            Boolean started = false;
            Boolean threw = false;
            try {
                Flow.Interview interview = Flow.Interview.createInterview(
                    'Meadow_RecordBinding',
                    new Map<String, Object>{'mode'=>mode, 'targetId'=>String.valueOf(target.Id)}
                );
                interview.start();
                started = true;
            } catch (Exception error) {
                threw = true;
                System.debug('FLOW_ERROR|'+mode+'|'+error.getTypeName()+'|'+error.getMessage());
            }
            Account stored = [SELECT Name, Description FROM Account WHERE Id=:target.Id];
            putCell(answers, firstCell, started);
            putCell(answers, firstCell+1, threw);
            putCell(answers, firstCell+2, stored.Name=='Meadow retained name');
            putCell(answers, firstCell+3, stored.Description=='Before');
            putCell(answers, firstCell+4, stored.Description=='After');
        } finally {
            Database.rollback(point);
        }
    }

    private static void putCell(Map<String, Boolean> answers, Integer cell, Boolean value) {
        String suffix = cell < 10 ? '0'+String.valueOf(cell) : String.valueOf(cell);
        answers.put('FRB'+suffix, value);
    }

    private static void observeTrigger(Map<String, Boolean> answers) {
        Savepoint point = Database.setSavepoint();
        try {
            Account target = new Account(Name='Meadow untouched trigger', Description='Before');
            Boolean inserted = false;
            try {
                insert target;
                inserted = true;
            } catch (Exception error) {
                System.debug('FLOW_ERROR|record_trigger_null|'+error.getTypeName()+'|'+error.getMessage());
            }
            Id targetId = target.Id;
            List<Account> stored = [SELECT Name, Description FROM Account WHERE Id=:targetId];
            answers.put('FRT01', inserted);
            answers.put('FRT02', stored.size()==1 && stored[0].Name=='Meadow untouched trigger');
            answers.put('FRT03', stored.size()==1 && stored[0].Description=='Before');
        } finally {
            Database.rollback(point);
        }
    }
}
