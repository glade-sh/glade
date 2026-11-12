public class PersonLookupBehavior {
    private static Id personType() {
        return Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId();
    }
    public static String lookup() {
        Account seed = new Account(RecordTypeId=personType(),FirstName='Robert',LastName='Hartwell');
        Account twin = new Account(RecordTypeId=personType(),FirstName='Bob',LastName='Hartwell');
        insert new List<Account>{seed,twin};
        StandardBehavior.remember(seed.Id,'seed');
        StandardBehavior.remember(twin.Id,'twin');
        String byId;
        try {
            List<String> rules = new List<String>();
            for (Datacloud.DuplicateResult d : Datacloud.FindDuplicatesByIds.findDuplicatesByIds(new List<Id>{seed.Id})[0].getDuplicateResults()) {
                List<String> hits = new List<String>();
                for (Datacloud.MatchResult m : d.getMatchResults()) for (Datacloud.MatchRecord r : m.getMatchRecords()) {
                    hits.add(r.getRecord().Id==twin.Id?'twin':r.getRecord().Id==seed.Id?'self':'unknown');
                }
                hits.sort();
                rules.add(d.getDuplicateRule()+'/'+d.isAllowSave()+'/['+String.join(hits,',')+']');
            }
            rules.sort(); byId=String.join(rules,'+');
        } catch (Exception e) { byId=e.getTypeName()+'/'+e.getMessage(); }
        return 'byId='+byId+'|inferred='+StandardBehavior.find(new Account(FirstName='Bob',LastName='Hartwell'))+
            '|business='+StandardBehavior.find(new Account(Name='Unrelated Industrial'));
    }
}
