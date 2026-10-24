public class PersonQueryCountQueryWithBinds {
    public static String observe() {
        return String.valueOf(Database.countQueryWithBinds('SELECT COUNT() FROM Account WHERE Loyalty__pc = null AND Id = null', new Map<String, Object>(), AccessLevel.SYSTEM_MODE));
    }
}
