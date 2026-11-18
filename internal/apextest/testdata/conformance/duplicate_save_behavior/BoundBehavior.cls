public class BoundBehavior {
    public static String uncapped() {
        List<Account> batch = new List<Account>();
        for (Integer i=0; i<105; i++) batch.add(new Account(Name='Test Account '+i,BillingCity='Northport'));
        insert batch;
        Map<Id,Integer> labels = new Map<Id,Integer>();
        for (Integer i=0;i<batch.size();i++) labels.put(batch[i].Id,i);
        Account candidate = new Account(Name='Test Account 106',BillingCity='Northport');
        Set<Integer> members = new Set<Integer>();
        Integer unexpected=0, found=0;
        for (Datacloud.DuplicateResult d : Datacloud.FindDuplicates.findDuplicates(new List<SObject>{candidate})[0].getDuplicateResults()) {
            for (Datacloud.MatchResult m : d.getMatchResults()) {
                found += m.getSize();
                for (Datacloud.MatchRecord r : m.getMatchRecords()) {
                    if (labels.containsKey(r.getRecord().Id)) members.add(labels.get(r.getRecord().Id));
                    else unexpected++;
                }
            }
        }
        List<Integer> ordered = new List<Integer>(members); ordered.sort();
        return 'size='+found+'|unexpected='+unexpected+'|members='+JSON.serialize(ordered);
    }
}
