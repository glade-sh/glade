public class ShapeOuter {
    public static ShapeOuter Instance = new ShapeOuter();
    public String Text = 'Canopy';
    // Compile-only: static field -> instance field -> platform method.
    public static void compileRoutes() {
        String direct = ShapeOuter.Instance.Text.toLowerCase();
        String qualified = System.String.valueOf(ShapeOuter.Instance.Text).toLowerCase();
    }
}
