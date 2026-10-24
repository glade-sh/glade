public class PersonQueryMissingField {
    public static String observe() {
        try { Database.query('SELECT Missing__pc FROM Account'); return 'unexpected'; } catch (QueryException e) { return 'missing'; }
    }
}
