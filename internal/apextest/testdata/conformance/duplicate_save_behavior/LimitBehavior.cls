public class LimitBehavior {
    private static String graph(Datacloud.DuplicateResult d, Map<Id,Integer> labels, List<Id> sortedIds) {
        List<Integer> hits = new List<Integer>(), ids = new List<Integer>();
        for (Datacloud.MatchResult m : d.getMatchResults()) {
            for (Datacloud.MatchRecord r : m.getMatchRecords()) {
                hits.add(labels.get(r.getRecord().Id)); ids.add(sortedIds.indexOf(r.getRecord().Id));
            }
        }
        hits.sort(); ids.sort();
        List<Integer> omitted=new List<Integer>(), ranks=new List<Integer>();
        for(Integer i=0; i<labels.size(); i++) {
            if(!hits.contains(i)) omitted.add(i);
            if(!ids.contains(i)) ranks.add(i);
        }
        return hits.size()+'/'+JSON.serialize(omitted)+'/'+JSON.serialize(ranks);
    }
    public static String lateExact() {
        List<Account> seeds=new List<Account>();
        for (Integer i=0; i<105;i++) seeds.add(new Account(Name='Test Account '+(104-i),BillingCity='Northport'));
        insert seeds;
        Map<Id,Integer> labels=new Map<Id,Integer>(); List<Id> sortedIds=new List<Id>();
        for (Integer i=0;i<seeds.size();i++) { labels.put(seeds[i].Id,i); sortedIds.add(seeds[i].Id); }
        sortedIds.sort();
        Account candidate=new Account(Name='Test Account 0',BillingCity='Northport');
        Datacloud.DuplicateResult result=Datacloud.FindDuplicates.findDuplicates(new List<Account>{candidate})[0].getDuplicateResults()[0];
        Database.SaveResult saved=Database.insert(candidate,false);
        return 'find='+graph(result,labels,sortedIds)+'|save='+String.valueOf(saved.getErrors()[0].getStatusCode())+'/'+graph(((Database.DuplicateError)saved.getErrors()[0]).getDuplicateResult(),labels,sortedIds);
    }
}
