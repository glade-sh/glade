public class ExplicitSort { public interface ChildComparable extends Comparable {}
 public class Direct implements ChildComparable, Comparable {
  public Integer value; public Direct(Integer n) { value = n; }
  public Integer compareTo(Object other) { return value - ((Direct)other).value; }
 }
 public static void run() { List<ChildComparable> a = new List<ChildComparable>{new Direct(3),new Direct(1),new Direct(2)}; a.sort(); System.assertEquals(1,((Direct)a[0]).value); } }
