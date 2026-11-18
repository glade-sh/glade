public virtual class StubOnlyParamCtor {
 public static Integer ctorCount=0;
 public Integer ctors=0;
 public StubOnlyParamCtor(String seed) { ctors++; ctorCount++; }
 public virtual String run(String value) { return 'real-param-ctor'; }
}
