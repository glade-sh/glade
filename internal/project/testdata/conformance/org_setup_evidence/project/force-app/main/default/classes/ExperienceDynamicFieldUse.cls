public class ExperienceDynamicFieldUse {
    public static String observe() {
        try { new User().get('IsPortalEnabled'); return 'available'; } catch (Exception e) { return e.getTypeName(); }
    }
}
