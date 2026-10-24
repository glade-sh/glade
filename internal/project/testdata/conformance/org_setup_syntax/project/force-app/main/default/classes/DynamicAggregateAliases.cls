public class DynamicAggregateAliases {
    public static String observe() {
        List<AggregateResult> a = Database.query('SELECT COUNT(Id) PersonEmail FROM Account WHERE Id = null'); List<AggregateResult> o = Database.query('SELECT COUNT(Id) CurrencyIsoCode FROM Opportunity WHERE Id = null'); return String.valueOf(a[0].get('PersonEmail')) + '/' + String.valueOf(o[0].get('CurrencyIsoCode'));
    }
}
