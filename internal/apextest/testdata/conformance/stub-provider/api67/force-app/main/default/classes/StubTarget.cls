public virtual class StubTarget {
 public static Integer staticCalls=0;
 public static Integer ctorCount=0;
 public static Integer privateStaticCalls=0;
 public Integer ctors=0;
 public Integer publicCalls=0;
 public Integer privateCalls=0;
 public Integer nonVirtualCalls=0;
 public StubTarget() { ctors++; ctorCount++; }
 public virtual String pub(String value) { publicCalls++; return 'real-pub'; }
 public String nonVirtual(String value) { nonVirtualCalls++; return 'real-nonvirtual'; }
 private String hidden(String value) { privateCalls++; return 'real-hidden'; }
 public String callHidden(String value) { return hidden(value); }
 private static String hiddenStatic(String value) { privateStaticCalls++; return 'real-hidden-static'; }
 public static String callHiddenStatic(String value) { return hiddenStatic(value); }
 public static String stat(String value) { staticCalls++; return 'real-stat'; }
}
