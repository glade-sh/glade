public class PersonQueryQuery {
    public static String observe() {
        return String.valueOf(Database.query('SELECT Loyalty__pc FROM Account WHERE Id = null').size());
    }
}
