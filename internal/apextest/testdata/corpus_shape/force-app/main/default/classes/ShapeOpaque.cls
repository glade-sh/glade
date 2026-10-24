public class ShapeOpaque {
    // Resolve an optional package type by name; no static dependency on its shape.
    public Object forward(Object request) {
        System.Type provider = System.Type.forName('cloudkit.Envelope');
        Object response = request;
        return response;
    }
}
