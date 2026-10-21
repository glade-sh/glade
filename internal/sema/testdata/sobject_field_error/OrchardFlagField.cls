public class OrchardFlagField {

    public static void run() {
        Contact record = new Contact(); record.HasOptedOutOfEmail.addError('invalid flag'); System.assertEquals(true, record.hasErrors());
    }
}
