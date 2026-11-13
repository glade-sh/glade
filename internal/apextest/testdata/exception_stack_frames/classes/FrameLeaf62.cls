public class FrameLeaf62 {

    public static void sameThrow() { throw new IllegalArgumentException('x'); }


    public static void sameNpe() { String s = null; Integer n = s.length(); }

    public static void npe() { String s = null; Integer n = s.length(); }
    public static void throwCustom() { throw new FrameCustomException('x'); }
    public static void throwIAE() { throw new IllegalArgumentException('x'); }

    public static Exception newDml() { return new DmlException('x'); }


    public static Exception newCustom() { return new FrameCustomException('x'); }

    public static String boom() { String s = null; return String.valueOf(s.length()); }
    public static void fail() { String s = null; Integer n = s.length(); }
}
