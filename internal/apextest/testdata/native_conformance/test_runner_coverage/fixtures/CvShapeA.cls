public class CvShapeA {
    public static Integer seed = 4;
    static {
        seed += 1;
    }
    public Integer member = 8;
    public Integer automatic { get; set; }
    public Integer explicitGetter {
        get {
            return member;
        }
    }
    public class NestedProbe {
        public Integer value(Integer n) {
            return n + 2;
        }
    }
    public static Integer run(Boolean left) {
        Integer declarationOnly;
        Integer initialized = 1;
        String folded = 'alpha'
            + 'beta';
        String dynamicText = folded
            + String.valueOf(seed);
        String chained = dynamicText
            .trim()
            .toUpperCase();
        Integer split = initialized
            + seed;
        System.debug('coverage-shape');
        if (left &&
            chained.length() > 0) {
            declarationOnly = split;
        } else {
            declarationOnly = 0;
        }
        return declarationOnly;
    }
    public static Integer sameLine(Boolean fail) {
        Integer n = 1; if (fail) { throw new ShapeException('shape'); } n += 1;
        return n;
    }
    public static Integer bump() {
        seed += 1;
        return seed;
    }
    public static void debugArgument() {
        System.debug(bump());
    }
    public static Integer mixedDebugLine() {
        Integer n = 1; System.debug(n); n++;
        return n;
    }
    public class ShapeException extends Exception {}
}
