public class OrchardRevenueField {

    public static void run() {
        Account record = new Account(AnnualRevenue=3); record.AnnualRevenue.addError('invalid revenue'); System.assertEquals(true, record.hasErrors()); System.assertEquals('AnnualRevenue', record.getErrors()[0].getFields()[0]);
    }
}
