public class OrchardNameField {

    public static void run() {
        Account record = new Account(Name='Orchard'); record.Name.addError('invalid name'); System.assertEquals(true, record.hasErrors());
    }
}
