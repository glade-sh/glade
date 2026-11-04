global virtual class CaptureOwner {
    private class Child extends CaptureOwner { private Child() {} }
    private class Other { private Other() {} }
    public class Boundary extends CaptureOwner { public Boundary() {} }
    private static String probe(Type t) {
        try { t.newInstance(); return 'constructed'; }
        catch (Exception e) { return e.getTypeName(); }
    }
    global static String run() {
        return probe(Child.class) + '|' + probe(Other.class) + '|' + probe(Boundary.class);
    }
}
