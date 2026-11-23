public class SubclassSort { public virtual class Parent implements Comparable {
  public Integer value; public Parent(Integer n) { value = n; }
  public Integer compareTo(Object other) { return value - ((Parent)other).value; }
 }
 public class Child extends Parent { public Child(Integer n) { super(n); } }
 public static void run() { List<Child> a = new List<Child>{new Child(3),new Child(1),new Child(2)}; a.sort(); System.assertEquals(1,a[0].value); } }
