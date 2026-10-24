public class TerritoryUse {
    public static String observe() {
        try { Database.query('SELECT Id FROM Territory2'); return 'available'; } catch (Exception e) { return e.getTypeName(); }
    }
}
