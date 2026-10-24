public class ShapeReturn {
    // The unconditional post-return slice is in anonymous.apex, not a named method.
    public static void compileRoute() {
        String text = ShapeOuter.Instance.Text.toLowerCase();
    }
}
