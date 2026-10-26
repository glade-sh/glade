public class ShapeSystemEnums {
    // Compile-only: platform enum constant name() and ordinal(), unqualified
    // and namespace-qualified, in top-level and inner classes.
    // Native basis: sema/testdata/enum_value_methods/org.tsv:64-67,144-147.
    public static String compileRoutes() {
        String quiddity = Quiddity.ANONYMOUS.name() + System.Quiddity.ANONYMOUS.ordinal();
        String token = JSONToken.START_OBJECT.name() + System.JSONToken.END_OBJECT.ordinal();
        String display = DisplayType.STRING.name() + Schema.DisplayType.BOOLEAN.ordinal();
        String level = System.LoggingLevel.ERROR.name() + System.LoggingLevel.WARN.ordinal();
        return quiddity + token + display + level + new Helper().observe();
    }

    private class Helper {
        String observe() {
            return Quiddity.SYNCHRONOUS.name() + System.Quiddity.SYNCHRONOUS.ordinal() +
                JSONToken.FIELD_NAME.name() + System.JSONToken.VALUE_STRING.ordinal() +
                DisplayType.PICKLIST.name() + Schema.DisplayType.DATE.ordinal() +
                System.LoggingLevel.DEBUG.name() + System.LoggingLevel.FINE.ordinal();
        }
    }
}
