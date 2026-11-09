public class Q3WrapperProbe62 {
 public class Box { public Q3Item__c item; public Box() { item=new Q3Item__c(When__c=Datetime.newInstanceGMT(2020,1,2,3,4,5)); } }
 public static void run() { Box b=new Box(); System.assertEquals(1577934245000L,b.item.When__c.getTime()); }
}
