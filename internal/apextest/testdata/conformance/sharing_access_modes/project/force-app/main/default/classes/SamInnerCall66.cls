public with sharing class SamInnerCall66 {
  public static String inner() { return new SamInner66.Inn().c(); }
  public static String innerW() { return new SamInner66.InW().c(); }
  public static String outer() { return new SamInner66().outer(); }
}
