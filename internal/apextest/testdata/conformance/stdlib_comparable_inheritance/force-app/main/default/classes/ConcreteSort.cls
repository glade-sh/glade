public class ConcreteSort { public interface ChildComparable extends Comparable {}
 public virtual class Indirect implements ChildComparable {
  public Integer value; public Indirect(Integer n) { value = n; }
  public Integer compareTo(Object other) { return value - ((Indirect)other).value; }
 }
public static void run() { List<Indirect> a = new List<Indirect>{new Indirect(3),new Indirect(1),new Indirect(2)}; a.sort(); System.assertEquals(1,a[0].value); } }
