@IsTest(SeeAllData = true) public class QueryProbeSeeAllDataTest {
    @IsTest static void seeAllData() {
        Map<String, Object> row = new Map<String, Object>();
        row.put('seeAllDataTrueCount', [SELECT COUNT() FROM Account WHERE Name = 'Glade Synthetic Existing Row']);
        System.assert(false, 'OBSERVED|seeAllData|' + JSON.serialize(row));
    }
}
