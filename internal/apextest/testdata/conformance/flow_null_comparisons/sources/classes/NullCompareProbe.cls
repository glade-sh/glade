public class NullCompareProbe {
    public class ObservationException extends Exception {}
    public static Map<String, Object> observe() {
        Map<String, Object> results = new Map<String, Object>();
        List<String> cases = new List<String>{'N01','N02','N03','N04','N05','N06','N07','N08','N09','N10','N11','N12','N13','N14','N15','N16','N17','N18','N19','N20','N21','N22','N23','N24','N25','N26','N27','N28','N29','N30','C01','C02','C03','C04','C05','C06','C07','C08','C09','C10','C11','C12','C13','C14','C15','C16','C17','C18','C19','C20','C21','C22','C23','C24','C25','C26','C27','C28','C29','C30'};
        List<Account> seeds = new List<Account>();
        for (String caseId : cases) {
            Flow.Interview interview = Flow.Interview.createInterview('NullCompareAuto', new Map<String,Object>{'CaseId' => caseId, 'InputRecord' => new Account()});
            interview.start();
            results.put('auto/' + caseId, interview.getVariableValue('Outcome'));
            seeds.add(new Account(Name = 'NullCompareSynthetic', AccountNumber = caseId));
        }
        insert seeds;
        Set<Id> ids = new Set<Id>();
        for (Account seed : seeds) { ids.add(seed.Id); }
        for (Account seed : [SELECT AccountNumber, Description FROM Account WHERE Id IN :ids]) {
            results.put('trigger/' + seed.AccountNumber, seed.Description);
        }
        return results;
    }
}
