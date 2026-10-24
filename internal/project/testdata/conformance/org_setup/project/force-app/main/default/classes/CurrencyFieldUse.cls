public class CurrencyFieldUse {
    public static String observe() {
        try { Database.query('SELECT CurrencyIsoCode FROM Opportunity'); return 'available'; } catch (Exception e) { return e.getTypeName(); }
    }
}
