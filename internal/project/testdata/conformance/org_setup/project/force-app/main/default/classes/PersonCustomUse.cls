public class PersonCustomUse {
    public static String observe() {
        try { new Account().get('Preference__pc'); return 'available'; } catch (Exception e) { return e.getTypeName(); }
    }
}
