public class Q3Bundle67 {
 private class Rank implements Comparable { Integer n; Rank(Integer x){n=x;} public Integer compareTo(Object other){return n-((Rank)other).n;} }
 private class Box { public Event a; Box(){a=new Event(); a.StartDateTime=Datetime.newInstanceGMT(2020,1,2,3,4,5);} }
 public static void run() {
  List<Rank> vals = new List<Rank>{new Rank(3),new Rank(1),new Rank(2)}; vals.sort(); System.assertEquals(1,vals[0].n); System.assertEquals(3,vals[2].n);
  Box b=new Box(); System.assertEquals(1577934245000L,b.a.StartDateTime.getTime());
  System.assertEquals('/apex/q3mixedcase',Page.Q3MixedCase.getUrl());
  Schema.SObjectField tok=Account.Name; System.assertEquals('Name',tok.getDescribe().getName());
 }
}