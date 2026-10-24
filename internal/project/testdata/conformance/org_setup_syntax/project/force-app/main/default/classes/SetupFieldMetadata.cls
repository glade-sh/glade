public class SetupFieldMetadata {
    public static String observe() {
        return String.valueOf(Account.SObjectType.getDescribe().fields.getMap().containsKey('PersonEmail')) + '/' + String.valueOf(Opportunity.SObjectType.getDescribe().fields.getMap().containsKey('CurrencyIsoCode'));
    }
}
