public class ItemCtrl {
    @AuraEnabled(cacheable=true)
    public static String getItems(String recordId) {
        return 'items:' + recordId;
    }
}
