public class Q3ListProbe62 {
 public class Rank implements Comparable { public Integer n; public Rank(Integer x) { n=x; } public Integer compareTo(Object o) { return n-((Rank)o).n; } }
 public static void run() { List<Comparable> a = new List<Comparable>{new Rank(3),new Rank(1),new Rank(2)}; a.sort(); System.assertEquals(1,((Rank)a[0]).n); System.assertEquals(3,((Rank)a[2]).n); }
}
