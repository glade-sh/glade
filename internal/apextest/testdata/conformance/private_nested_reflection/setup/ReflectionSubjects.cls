public class ReflectionSubjects {
 private class PrivateChild extends ReflectionProbeBase {}
 public class PublicChild extends ReflectionProbeBase {}
 public static Object privateValue() { return new PrivateChild(); }
 public static Object publicValue() { return new PublicChild(); }
}
