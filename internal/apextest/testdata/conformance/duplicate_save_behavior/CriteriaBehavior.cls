public class CriteriaBehavior {
    private static String graph(Datacloud.DuplicateResult rule, Map<Id,String> labels) {
        List<String> found = new List<String>();
        for (Datacloud.MatchResult result : rule.getMatchResults()) {
            for (Datacloud.MatchRecord match : result.getMatchRecords()) {
                System.assert(labels.containsKey(match.getRecord().Id), 'unknown candidate');
                found.add(labels.get(match.getRecord().Id));
            }
        }
        found.sort();
        return rule.getDuplicateRule() + '/' + String.valueOf(rule.isAllowSave()) + '[' + String.join(found, ',') + ']';
    }
    private static String problem(Database.Error[] errors, Map<Id,String> labels) {
        System.assertEquals(1, errors.size());
        Database.Error error = errors[0];
        System.assert(error instanceof Database.DuplicateError);
        return String.valueOf(error.getStatusCode()) + '/' + graph(((Database.DuplicateError)error).getDuplicateResult(), labels);
    }
    private static String saved(Database.SaveResult result, Map<Id,String> labels) {
        return result.isSuccess() ? 'saved' : problem(result.getErrors(), labels);
    }
    public static String editGate() {
        Contact seed = new Contact(LastName='Aster',Title='Qualifies',Phone='4155551234');
        Contact existing = new Contact(LastName='Birch',Title='Qualifies',Phone='4155551235');
        Contact other = new Contact(LastName='Decoy',Title='Qualifies',Phone='2028410150');
        Contact outside = new Contact(LastName='Windrush',Title='Outside',Phone='4155551236');
        insert new List<Contact>{seed, existing, other, outside};
        Map<Id,String> labels = new Map<Id,String>{seed.Id=>'seed',existing.Id=>'existing',other.Id=>'other',outside.Id=>'outside'};
        List<String> out = new List<String>();
        Contact candidate = new Contact(LastName='Quillstone',Title='Qualifies',Phone='4155551237');
        List<Datacloud.DuplicateResult> rules = Datacloud.FindDuplicates.findDuplicates(new List<Contact>{candidate})[0].getDuplicateResults();
        for (Datacloud.DuplicateResult rule : rules) {
            if (rule.getDuplicateRule()=='ProbeTitlePhoneBlock') out.add('find=' + graph(rule,labels));
        }
        System.assertEquals(1,out.size(),'expected custom rule');
        out.add('insert=' + saved(Database.insert(candidate,false),labels));
        out.add('unchanged=' + saved(Database.update(new Contact(Id=existing.Id,Description='Unrelated'),false),labels));
        Database.UpsertResult changed = Database.upsert(new Contact(Id=other.Id,Title='Qualifies',Phone='4155551238'),false);
        out.add('changedUpsert=' + (changed.isSuccess() ? 'saved/' + changed.isCreated() : problem(changed.getErrors(),labels)));
        out.add('enter=' + saved(Database.update(new Contact(Id=outside.Id,Title='Qualifies'),false),labels));
        out.add('leave=' + saved(Database.update(new Contact(Id=existing.Id,Title='Outside'),false),labels));
        return String.join(out,';');
    }
}
