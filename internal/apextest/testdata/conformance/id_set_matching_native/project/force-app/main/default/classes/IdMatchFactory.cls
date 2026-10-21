public class IdMatchFactory {
    private static IdMatchToken saved;
    public static void setToken(IdMatchToken value) { saved = value; }
    public static IdMatchToken getToken() { return saved; }
}
