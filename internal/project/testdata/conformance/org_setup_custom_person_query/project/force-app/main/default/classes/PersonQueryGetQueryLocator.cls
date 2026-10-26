public class PersonQueryGetQueryLocator {
    public static String observe() {
        return String.valueOf(Database.getQueryLocator('SELECT Loyalty__pc FROM Account WHERE Id = null').iterator().hasNext());
    }
}
