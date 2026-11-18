public class OrderingSource64 {
public static String run() {
    insert new List<OrderingItem__c>{new OrderingItem__c(Name='v0',Rank__c='Zulu'),new OrderingItem__c(Name='v1',Rank__c='Alpha'),new OrderingItem__c(Name='v2',Rank__c='Middle'),new OrderingItem__c(Name='v3')};
    List<String> result = new List<String>();
    for (String clause : new List<String>{'ASC','DESC','ASC NULLS FIRST','ASC NULLS LAST','DESC NULLS FIRST','DESC NULLS LAST'}) {
        List<String> names = new List<String>();
        for (OrderingItem__c r : Database.query('SELECT Name FROM OrderingItem__c ORDER BY Rank__c '+clause+', Id ASC')) names.add(r.Name);
        result.add(clause+':'+String.join(names,','));
    }
    return String.join(result,'|');
}
}