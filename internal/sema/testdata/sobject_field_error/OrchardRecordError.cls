public class OrchardRecordError {

    public static void run() {
        Account record = new Account(); record.addError('invalid record'); System.assertEquals(true, record.hasErrors());
    }
}
