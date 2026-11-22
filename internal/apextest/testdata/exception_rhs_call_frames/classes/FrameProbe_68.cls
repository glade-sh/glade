public class FrameProbe_68 {
    public static Integer fail() { throw new FrameException(); }
    public static Integer wrap(Integer value) { return value; }
    public class FrameException extends Exception {}
}
