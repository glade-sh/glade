public with sharing class SamInnerCall68 {
  public static String inner() { return new SamInner68.Inn().c(); }
  public static String innerW() { return new SamInner68.InW().c(); }
  public static String outer() { return new SamInner68().outer(); }
}
