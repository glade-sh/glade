public abstract class ShapeInnerTargets {
    // Native: inner_type_shapes/org.tsv:34-41,64-67.
    public Boolean matches() { return this instanceof Leaf; }
    public static Boolean castTarget(ShapeInnerTargets value) { return ((Leaf) value) != null; }
    public class Leaf extends ShapeInnerTargets {}
}
