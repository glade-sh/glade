@IsTest
private class OrdRouteTest {
    @IsTest
    static void defaultOrder() {
        insert new List<Ord__c>{OrdSeed.row(7), OrdSeed.row(2), OrdSeed.row(11), OrdSeed.row(4)};
        insert new List<Ord__c>{OrdSeed.row(1), OrdSeed.row(9), OrdSeed.row(12), OrdSeed.row(3)};
        insert new List<Ord__c>{OrdSeed.row(6), OrdSeed.row(10), OrdSeed.row(5), OrdSeed.row(8)};
        List<String> rs = new List<String>();
        for (Ord__c r : [SELECT Rnk__c FROM Ord__c]) {
            rs.add(String.valueOf(r.Rnk__c));
        }
        System.assert(false, 'r=' + String.join(rs, ','));
    }
}
