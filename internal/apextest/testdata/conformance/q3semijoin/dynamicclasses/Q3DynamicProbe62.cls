public class Q3DynamicProbe62 {
 public static void run() { List<Q3DynamicRank62> a = new List<Q3DynamicRank62>();
 for (Integer n : new List<Integer>{3,1,2}) { Q3DynamicRank62 x=(Q3DynamicRank62)Type.forName('Q3DynamicRank62').newInstance(); x.n=n; a.add(x); }
 a.sort(); System.assertEquals(1,a[0].n); System.assertEquals(3,a[2].n); }
}
