public class DollarBindProbe {
    public static String observe(String key) {
        try {
            List<SObject> rows = Database.queryWithBinds('SELECT Id FROM Account WHERE Name = :' + key,
                new Map<String,Object>{key => 'Aster'}, AccessLevel.USER_MODE);
            return 'ok|' + rows.size();
        } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    }
}
