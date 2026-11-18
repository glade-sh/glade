public class XLeaf62 {
    public static void x_throw_dml() { throw new DmlException('x'); }
    public static void x_throw_npe() { throw new NullPointerException('x'); }
    public static Exception x_new_iae() { return new IllegalArgumentException('x'); }
    public static Exception x_new_npe() { return new NullPointerException('x'); }
}
