public class NullRouteProbe {
    public class ObservationException extends Exception {}
    public static List<String> cases() { return new List<String>{'N01','N02','N03','N04','N05','N06','N07','N08','N09','N10','N11','N12','N13','N14','N15','N16','N17','N18','N19','N20','N21','N22','N23','N24','N25','N26','N27','N28','N29','N30','C01','C02','C03','C04','C05','C06','C07','C08','C09','C10','C11','C12','C13','C14','C15','C16','C17','C18','C19','C20','C21','C22','C23','C24','C25','C26','C27','C28','C29','C30'}; }
    public static String observe(String route) {
        Map<String, Object> results = new Map<String, Object>();
        if (route == 'typed' || route == 'action' || route == 'subflow') {
            for (String caseId : cases()) {
                Map<String, Object> inputs = new Map<String, Object>{'CaseId' => caseId, 'InputRecord' => new Account()};
                Object outcome;
                if (route == 'action') {
                    Invocable.Action action = Invocable.Action.createCustomAction('flow', 'NullCompareAuto');
                    action.setInvocations(new List<Map<String, Object>>{inputs});
                    Invocable.Action.Result result = action.invoke()[0];
                    System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
                    outcome = result.getOutputParameters().get('Outcome');
                } else {
                    Flow.Interview interview;
                    if (route == 'typed') interview = new Flow.Interview.NullCompareAuto(inputs);
                    else interview = Flow.Interview.createInterview('NullRouteParent', inputs);
                    interview.start();
                    outcome = interview.getVariableValue('Outcome');
                }
                results.put(caseId, outcome);
            }
            return JSON.serialize(results);
        }
        Boolean beforeSave = route.startsWith('before_') && route != 'before_delete';
        String marker = beforeSave ? 'NullRouteBefore' : (route == 'before_delete' ? 'NullRouteDelete' : 'NullRouteAfter');
        if (!beforeSave) delete [SELECT Id FROM Contact WHERE LastName = :marker];
        List<Account> seeds = new List<Account>();
        Boolean existing = route.endsWith('_update') || route == 'before_delete';
        for (String caseId : cases()) seeds.add(new Account(Name = existing ? 'NullRouteSeed' : marker, AccountNumber = caseId));
        if (existing) {
            insert seeds;
            for (Account seed : seeds) seed.Name = marker;
        }
        if (route == 'before_delete') {
            update seeds;
            delete seeds;
        } else if (route.contains('upsert')) upsert seeds;
        else if (existing) update seeds;
        else insert seeds;
        if (beforeSave) {
            Set<Id> ids = new Set<Id>();
            for (Account seed : seeds) ids.add(seed.Id);
            for (Account seed : [SELECT AccountNumber, Description FROM Account WHERE Id IN :ids]) results.put(seed.AccountNumber, seed.Description);
        } else {
            for (Contact row : [SELECT Department, Description FROM Contact WHERE LastName = :marker]) results.put(row.Department, row.Description);
        }
        return JSON.serialize(results);
    }
    public static String asyncRead() {
        Map<String, Object> results = new Map<String, Object>();
        for (String caseId : cases()) results.put(caseId, 'pending');
        for (Contact row : [SELECT Department, Description FROM Contact WHERE LastName = 'NullRouteAsync']) results.put(row.Department, row.Description);
        return JSON.serialize(results);
    }
    public static void enqueue() {
        delete [SELECT Id FROM Contact WHERE LastName = 'NullRouteAsync'];
        List<Account> seeds = new List<Account>();
        for (String caseId : cases()) seeds.add(new Account(Name = 'NullRouteAsync', AccountNumber = caseId));
        insert seeds;
    }
}
