public class GardenFieldRead62 {
    public static String readMarker() {
        Contact contact = new Contact(LastName='Row', Marker__c='seed');
        return Contact.Marker__c;
    }
    public static String assignMarker() {
        Contact contact = new Contact(LastName='Row');
        Contact.Marker__c = 'sprout';
        return contact.Marker__c;
    }
    public static String lowerCaseControl() {
        Contact contact = new Contact(LastName='Row', Marker__c='control');
        return contact.Marker__c;
    }
}
