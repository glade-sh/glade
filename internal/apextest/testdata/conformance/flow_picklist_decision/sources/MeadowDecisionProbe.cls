public class MeadowDecisionProbe {
    public static void seed() {
        insert new List<Account>{
            new Account(Name='Meadow existing changed', Rating='Cold', Description='Initial'),
            new Account(Name='Meadow existing unchanged', Rating='Hot', Description='Initial'),
            new Account(Name='Meadow existing other', Rating='Cold', Description='Initial')
        };
    }

    public static Map<String, String> observe() {
        Savepoint point = Database.setSavepoint();
        try {
            Map<String, String> answers = new Map<String, String>();
            Account changed = [SELECT Id, Rating FROM Account WHERE Name='Meadow existing changed'];
            changed.Rating = 'Hot';
            update changed;
            answers.put('D01', description(changed.Id));

            Account unchanged = [SELECT Id, Rating FROM Account WHERE Name='Meadow existing unchanged'];
            unchanged.Rating = 'Hot';
            update unchanged;
            answers.put('D02', description(unchanged.Id));

            Account other = [SELECT Id, Rating FROM Account WHERE Name='Meadow existing other'];
            other.Rating = 'Warm';
            update other;
            answers.put('D03', description(other.Id));

            Account created = new Account(Name='Meadow created hot', Rating='Hot', Description='Initial');
            insert created;
            answers.put('D04', description(created.Id));

            Account sameTransaction = new Account(Name='Meadow created then changed', Rating='Cold', Description='Initial');
            insert sameTransaction;
            sameTransaction.Rating = 'Hot';
            update sameTransaction;
            answers.put('D05', description(sameTransaction.Id));
            return answers;
        } finally {
            Database.rollback(point);
        }
    }

    private static String description(Id accountId) {
        Account row = [SELECT Description FROM Account WHERE Id=:accountId];
        return row.Description;
    }
}
