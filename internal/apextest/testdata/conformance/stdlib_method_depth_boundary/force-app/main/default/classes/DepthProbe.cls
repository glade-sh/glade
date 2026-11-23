public class DepthProbe { public static Integer depth(Integer n) { if (n == 0) return 0; return 1 + depth(n - 1); } }
