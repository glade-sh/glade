public class PersonFlagUse {
    public static String observe() {
        try { Database.query('SELECT IsPersonAccount FROM Account'); return 'available'; } catch (Exception e) { return e.getTypeName(); }
    }
}
