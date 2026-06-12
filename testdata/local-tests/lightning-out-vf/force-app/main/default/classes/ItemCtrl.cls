public class ItemCtrl {
    @AuraEnabled(cacheable=true)
    public static List<Account> getItems(String recordId) {
        return new List<Account>{
            new Account(Id = recordId, Name = 'Local Widget')
        };
    }
}
