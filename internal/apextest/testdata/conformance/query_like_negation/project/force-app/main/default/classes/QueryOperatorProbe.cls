public class QueryOperatorProbe {
    public static String postfix() { return outcome('SELECT Id FROM Account WHERE Name NOT LIKE \'z%\''); }
    public static String lowercase() { return outcome('SELECT Id FROM Account WHERE Name not like \'z%\''); }
    public static String prefix() { return outcome('SELECT Id FROM Account WHERE (NOT Name LIKE \'z%\') AND Id = null'); }
    public static String notIn() { return outcome('SELECT Id FROM Account WHERE Name NOT IN (\'z\') AND Id = null'); }
    private static String outcome(String query) {
        try { Database.query(query); return 'accepted'; }
        catch (QueryException error) { return error.getTypeName() + ':' + error.getMessage(); }
    }
}
