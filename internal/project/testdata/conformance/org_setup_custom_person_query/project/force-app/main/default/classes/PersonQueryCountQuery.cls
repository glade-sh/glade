public class PersonQueryCountQuery {
    public static String observe() {
        return String.valueOf(Database.countQuery('SELECT COUNT() FROM Account WHERE Loyalty__pc = null AND Id = null'));
    }
}
