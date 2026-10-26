public class PersonQueryStaticAggregate {
    public static String observe() {
        return String.valueOf([SELECT COUNT(Loyalty__pc) total FROM Account WHERE Id = null][0].get('total'));
    }
}
