public class OrchardWrongArgument {

    public static void run() {
        Account record = new Account(); record.AnnualRevenue.addError(3);
    }
}
