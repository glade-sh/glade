public class PersonQueryStaticSelect {
    public static String observe() {
        return String.valueOf([SELECT Loyalty__pc FROM Account WHERE Id = null].size());
    }
}
