public class PersonContactUse {
    public static String observe() {
        try { Database.query('SELECT PersonContactId FROM Account'); return 'available'; } catch (Exception e) { return e.getTypeName(); }
    }
}
