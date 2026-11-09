public class TWDeep {
  public static void sys() { System.assert(false, 'deep sys msg'); }
  public static void api() { Assert.isTrue(false, 'deep api msg'); }
  public static void inner() { Assert.areEqual(1, 2, 'deep inner msg'); }
  public static void outer() { inner(); }
  public static void thrower() { throw new TWException('deep throw msg'); }
}