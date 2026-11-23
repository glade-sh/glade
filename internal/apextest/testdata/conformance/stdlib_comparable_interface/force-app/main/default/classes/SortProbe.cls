public class SortProbe {
 public interface Extended extends Comparable {}
 public interface Separate {}
 public class ExtendedValue implements Extended {
  public Integer value;
  public ExtendedValue(Integer n) { value = n; }
  public Integer compareTo(Object other) { return value - ((ExtendedValue)other).value; }
 }
 public class SeparateValue implements Separate, Comparable {
  public Integer value;
  public SeparateValue(Integer n) { value = n; }
  public Integer compareTo(Object other) { return value - ((SeparateValue)other).value; }
 }
 public class PlainValue { public Integer value; public PlainValue(Integer n) { value = n; } }
 public static void extended() {
  List<Extended> values = new List<Extended>{new ExtendedValue(3),new ExtendedValue(1),new ExtendedValue(2)};
  values.sort(); System.assertEquals(1, ((ExtendedValue)values[0]).value); System.assertEquals(3, ((ExtendedValue)values[2]).value);
 }
 public static void separate() {
  List<Separate> values = new List<Separate>{new SeparateValue(3),new SeparateValue(1),new SeparateValue(2)};
  values.sort(); System.assertEquals(1, ((SeparateValue)values[0]).value); System.assertEquals(3, ((SeparateValue)values[2]).value);
 }
 public static void noncomparable() {
  List<Object> values = new List<Object>{new PlainValue(2),new PlainValue(1)};
  Boolean caught = false;
  try { values.sort(); } catch (ListException e) { caught = true; System.assertEquals('One or more of the items in this list is not Comparable.', e.getMessage()); }
  System.assert(caught);
 }
}
