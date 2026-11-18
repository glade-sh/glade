public with sharing class SamInnerCall62 {
  public static String inner() { return new SamInner62.Inn().c(); }
  public static String innerW() { return new SamInner62.InW().c(); }
  public static String outer() { return new SamInner62().outer(); }
}
