public class RecordTypeDescribeUse {
    public static String observe() {
        return String.valueOf(Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('Synthetic_Client') == null);
    }
}
