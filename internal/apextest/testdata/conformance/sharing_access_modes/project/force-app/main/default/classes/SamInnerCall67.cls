public with sharing class SamInnerCall67 {
  public static String inner() { return new SamInner67.Inn().c(); }
  public static String innerW() { return new SamInner67.InW().c(); }
  public static String outer() { return new SamInner67().outer(); }
}
