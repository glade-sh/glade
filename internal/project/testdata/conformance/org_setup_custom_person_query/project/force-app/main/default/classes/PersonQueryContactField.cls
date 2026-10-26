public class PersonQueryContactField {
    public static String observe() {
        return String.valueOf([SELECT Loyalty__c FROM Contact WHERE Id = null].size());
    }
}
