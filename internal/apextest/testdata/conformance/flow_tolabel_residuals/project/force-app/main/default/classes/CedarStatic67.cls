public class CedarStatic67 {
    public static String run() {
        List<String> p = new List<String>();
        for (AggregateResult a : [SELECT COUNT(Id) c, toLabel(Cedar_Pick__c) lbl FROM Cedar_Report__c GROUP BY cedar_pick__c]) p.add(String.valueOf(a.getPopulatedFieldsAsMap()));
        p.sort();
        return 'api=67.0 static=[' + String.join(p, ';') + ']';
    }
}
