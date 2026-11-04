public virtual class Case245ReflectionCapture {
 private class Child extends Case245ReflectionCapture { private Child() {} }
 private class Other { private Other() {} }
 public class PublicChild extends Case245ReflectionCapture { private PublicChild() {} }
 public class Boundary { public Boundary() {} }
 private static String attempt(Type t) {
  try { Object v = t.newInstance(); return v == null ? 'null' : 'constructed'; }
  catch (Exception e) { return e.getTypeName(); }
 }
 public static Map<String,String> observe() {
  return new Map<String,String>{'privateSubclass'=>attempt(Child.class),'privateOther'=>attempt(Other.class),'publicSubclass'=>attempt(PublicChild.class),'publicBoundary'=>attempt(Boundary.class)};
 }
}
