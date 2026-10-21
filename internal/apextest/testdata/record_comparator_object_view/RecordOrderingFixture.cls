public class RecordOrderingFixture {
 public class AmountDescending implements System.Comparator<Opportunity> {
  public Integer compare(Opportunity left, Opportunity right) {
   return left.Amount==right.Amount ? 0 : (left.Amount>right.Amount ? -1 : 1);
  }
 }
 public class DecimalAscending implements System.Comparator<Decimal> {
  public Integer compare(Decimal left, Decimal right) {
   return left==right ? 0 : (left<right ? -1 : 1);
  }
 }
 public class FieldAscending implements System.Comparator<SObject> {
  private System.Comparator<Object> scalarOrder=(System.Comparator<Object>)new DecimalAscending();
  public Integer compare(SObject left, SObject right) {
   return scalarOrder.compare(left.get(Opportunity.Amount),right.get(Opportunity.Amount));
  }
 }
 public class PlainOrder {}
 public static void sortRecords(List<Opportunity> values,System.Comparator<Object> order) {
  values.sort(order);
 }
}
