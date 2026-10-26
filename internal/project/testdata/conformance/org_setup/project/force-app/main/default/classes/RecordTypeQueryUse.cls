public class RecordTypeQueryUse {
    public static String observe() {
        return String.valueOf([SELECT Id FROM RecordType WHERE SObjectType = 'Account' AND DeveloperName = 'Synthetic_Client'].size());
    }
}
