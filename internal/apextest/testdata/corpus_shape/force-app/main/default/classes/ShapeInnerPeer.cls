public abstract class ShapeInnerPeer {
    // Native: inner_type_shapes/org.tsv:64-67; the short name stays lexical.
    public Boolean matches() { return this instanceof Leaf; }
    public class Leaf extends ShapeInnerPeer {}
}
