public class RecordTypeAliasUse {
    public static String observe() {
        return String.valueOf([SELECT rt.Id FROM RecordType rt WHERE rt.SObjectType = 'Account' AND rt.DeveloperName = 'Synthetic_Client' AND rt.CreatedBy.Name = 'Synthetic_Author'].size());
    }
}
