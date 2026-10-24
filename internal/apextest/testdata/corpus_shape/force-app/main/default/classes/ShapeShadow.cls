public class ShapeShadow {
    public class Request { public String Text = 'local'; }
    // Native: conformance/inner_type_shapes/org.tsv:18-21.
    public enum Type { A, B }
    public static String typeName() { return Type.B.name(); }
    public enum EventBus { publish, hold }
    // Compile-only: lexical EventBus.publish is an enum value, not the platform method.
    public static void compileRoutes() {
        Request local = new Request();
        String text = local.Text.toLowerCase();
        String eventName = EventBus.publish.name();
        System.Request platform = System.Request.getCurrent();
    }
}
