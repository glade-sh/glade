public class PersonQueryStaticOrder {
    public static String observe() {
        return String.valueOf([SELECT Id FROM Account WHERE Id = null ORDER BY Loyalty__pc].size());
    }
}
