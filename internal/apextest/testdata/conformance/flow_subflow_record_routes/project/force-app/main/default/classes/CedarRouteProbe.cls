public class CedarRouteProbe {
    public static String invokeParent(Contact row, Boolean useInvocable) {
        if (useInvocable) {
            Invocable.Action action = Invocable.Action.createCustomAction('flow', 'CedarParent');
            action.setInvocationParameter('target', row);
            List<Invocable.Action.Result> results = action.invoke();
            System.assertEquals(1, results.size());
            System.assertEquals(true, results[0].isSuccess(), JSON.serialize(results[0].getErrors()));
            return (String)results[0].getOutputParameters().get('result');
        }
        Flow.Interview interview = Flow.Interview.createInterview('CedarParent', new Map<String,Object>{'target'=>row});
        interview.start();
        return (String)interview.getVariableValue('result');
    }
    public static String observe(String route, String operation) {
        Account first = new Account(Name='Cedar first');
        Account second = new Account(Name='Cedar second');
        insert new List<Account>{first,second};
        Account branch = new Account(Name='Cedar branch', ParentId=first.Id);
        insert branch;
        Contact row = new Contact(LastName='Cedar contact', AccountId=branch.Id);
        Boolean standalone = route == 'invocable' || route == 'interview';
        if (operation == 'insert' && !standalone) row.Department=route;
        insert row;
        String initial = [SELECT Description FROM Contact WHERE Id=:row.Id].Description;
        if (operation == 'update') {
            branch.ParentId=second.Id;
            update branch;
            if (!standalone) row.Department=route;
            update row;
        }
        String returned = null;
        if (standalone) {
            returned=invokeParent(row,route=='invocable');
            update new Contact(Id=row.Id,Description=returned);
        }
        Contact stored = [SELECT Description FROM Contact WHERE Id=:row.Id];
        return JSON.serialize(new Map<String,Object>{'initial'=>initial,'description'=>stored.Description,'returned'=>returned});
    }
}
