public class RecordTypeRelatedUse {
    public static String observe() {
        return String.valueOf([SELECT Id FROM RecordType WHERE SObjectType = 'Account' AND DeveloperName = 'Synthetic_Client' AND CreatedBy.Name = 'Synthetic_Author'].size());
    }
}
