public class PersonQueryQueryWithBinds {
    public static String observe() {
        return String.valueOf(Database.queryWithBinds('SELECT Loyalty__pc FROM Account WHERE Id = null', new Map<String, Object>(), AccessLevel.SYSTEM_MODE).size());
    }
}
