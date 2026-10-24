public class ExperienceUserUse {
    public static String observe() {
        try { Database.query('SELECT Id FROM User WHERE IsPortalEnabled = true'); return 'available'; } catch (Exception e) { return e.getTypeName(); }
    }
}
