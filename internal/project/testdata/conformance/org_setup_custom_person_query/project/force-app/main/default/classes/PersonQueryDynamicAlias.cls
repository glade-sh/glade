public class PersonQueryDynamicAlias {
    public static String observe() {
        return String.valueOf(Database.query('SELECT COUNT(Id) Loyalty__pc FROM Account WHERE Id = null')[0].get('Loyalty__pc'));
    }
}
