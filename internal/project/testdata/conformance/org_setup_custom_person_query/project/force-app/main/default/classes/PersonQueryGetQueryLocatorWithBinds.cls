public class PersonQueryGetQueryLocatorWithBinds {
    public static String observe() {
        return String.valueOf(Database.getQueryLocatorWithBinds('SELECT Loyalty__pc FROM Account WHERE Id = null', new Map<String, Object>(), AccessLevel.SYSTEM_MODE).iterator().hasNext());
    }
}
