public class ShapeLevelShadow {
    // Compile-only: explicit System.LoggingLevel enum values beside a project
    // LoggingLevel class. Native basis: sema/testdata/enum_value_methods/org.tsv:64-67,72-77,144-147.
    public static Map<String, Object> compileRoutes() {
        Map<String, Object> input = new Map<String, Object>();
        input.put('level', System.LoggingLevel.ERROR.name());
        input.put('ordinal', System.LoggingLevel.WARN.ordinal());
        input.put('nested', new Helper().observe());
        return input;
    }

    private class Helper {
        String observe() {
            return System.LoggingLevel.DEBUG.name() + System.LoggingLevel.FINE.ordinal();
        }
    }
}
