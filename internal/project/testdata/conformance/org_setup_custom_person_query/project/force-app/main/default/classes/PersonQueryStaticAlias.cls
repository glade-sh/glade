public class PersonQueryStaticAlias {
    public static String observe() {
        return String.valueOf([SELECT COUNT(Id) Loyalty__pc FROM Account WHERE Id = null][0].get('Loyalty__pc'));
    }
}
