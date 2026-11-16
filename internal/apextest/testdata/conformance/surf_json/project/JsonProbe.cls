public class JsonProbe {
 public class Plain {
  public String zeta = 'z';
  public String alpha = 'a';
  public Integer num;
  public String nulled = null;
 }
 public virtual class Base {
  public String baseField = 'b';
  public Integer baseNum = 7;
 }
 public class Derived extends Base {
  public String derivedField = 'd';
  public transient String secret = 's';
  public String tail;
 }
 public class OnlyTransient {
  public transient String secret = 's';
  public String kept = 'k';
  public transient Integer also = 3;
 }
 public class Nested {
  public Plain child;
  public List<Plain> items;
  public Map<String,Object> bag;
 }
 public class Cyclic {
  public String name = 'c';
  public Cyclic next;
 }

 public static String plainClass() { return JSON.serialize(new Plain()); }
 public static String plainNulls() { return JSON.serialize(new Plain(), true); }
 public static String derived() { return JSON.serialize(new Derived()); }
 public static String transientOnly() { return JSON.serialize(new OnlyTransient()); }
 public static String transientPretty() { return JSON.serializePretty(new OnlyTransient()); }
 public static String transientSuppressed() { return JSON.serialize(new OnlyTransient(), true); }
 public static String nested() {
  Nested n = new Nested();
  n.child = new Plain();
  n.items = new List<Plain>{ new Plain(), null };
  n.bag = new Map<String,Object>{'k'=>1, 'n'=>null};
  return JSON.serialize(n);
 }
 public static String nestedSuppressed() {
  Nested n = new Nested();
  n.child = new Plain();
  n.items = new List<Plain>{ new Plain(), null };
  n.bag = new Map<String,Object>{'k'=>1, 'n'=>null};
  return JSON.serialize(n, true);
 }
 public static String cyclic() {
  Cyclic c = new Cyclic();
  c.next = c;
  return JSON.serialize(c);
 }
}
