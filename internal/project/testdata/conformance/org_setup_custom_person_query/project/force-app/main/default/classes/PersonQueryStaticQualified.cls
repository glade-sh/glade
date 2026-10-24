public class PersonQueryStaticQualified {
    public static String observe() {
        return String.valueOf([SELECT a.Loyalty__pc FROM Account a WHERE Id = null].size());
    }
}
