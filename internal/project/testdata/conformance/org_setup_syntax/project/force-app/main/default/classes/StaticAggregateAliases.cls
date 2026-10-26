public class StaticAggregateAliases {
    public static String observe() {
        AggregateResult a = [SELECT COUNT(Id) PersonEmail FROM Account WHERE Id = null]; AggregateResult o = [SELECT COUNT(Id) CurrencyIsoCode FROM Opportunity WHERE Id = null]; return String.valueOf(a.get('PersonEmail')) + '/' + String.valueOf(o.get('CurrencyIsoCode'));
    }
}
