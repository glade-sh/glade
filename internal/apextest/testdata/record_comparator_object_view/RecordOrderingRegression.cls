@IsTest private class RecordOrderingRegression {
 private static List<Opportunity> rows() {
  return new List<Opportunity>{new Opportunity(Name='Lesser',Amount=2),new Opportunity(Name='Greater',Amount=9)};
 }
 @IsTest static void concreteObjectViewSort() {
  List<Opportunity> values=rows();
  System.Comparator<Object> order=(System.Comparator<Object>)new RecordOrderingFixture.AmountDescending();
  System.assert(order instanceof RecordOrderingFixture.AmountDescending);
  System.assertEquals(1,order.compare(values[0],values[1]));
  RecordOrderingFixture.sortRecords(values,order);
  System.assertEquals(9,values[0].Amount);
  System.assertEquals(2,values[1].Amount);
 }
 @IsTest static void sObjectObjectViewSort() {
  List<Opportunity> values=new List<Opportunity>{new Opportunity(Amount=9),new Opportunity(Amount=2)};
  Object held=new RecordOrderingFixture.FieldAscending();
  System.Comparator<Object> order=(System.Comparator<Object>)held;
  System.assert(order instanceof RecordOrderingFixture.FieldAscending);
  System.assertEquals(1,order.compare(values[0],values[1]));
  RecordOrderingFixture.sortRecords(values,order);
  System.assertEquals(2,values[0].Amount);
  System.assertEquals(9,values[1].Amount);
 }
 @IsTest static void typedConcreteSortControl() {
  List<Opportunity> values=rows();
  values.sort(new RecordOrderingFixture.AmountDescending());
  System.assertEquals(9,values[0].Amount);
 }
 @IsTest static void typedSObjectSortControl() {
  List<Opportunity> values=new List<Opportunity>{new Opportunity(Amount=9),new Opportunity(Amount=2)};
  values.sort(new RecordOrderingFixture.FieldAscending());
  System.assertEquals(2,values[0].Amount);
 }
 @IsTest static void primitiveObjectViewControl() {
  List<Decimal> values=new List<Decimal>{9,2};
  System.Comparator<Object> order=(System.Comparator<Object>)new RecordOrderingFixture.DecimalAscending();
  values.sort(order);
  System.assertEquals(2,values[0]);
 }
 @IsTest static void nonComparatorControl() {
  Object held=new RecordOrderingFixture.PlainOrder();
  try {
   System.Comparator<Object> order=(System.Comparator<Object>)held;
   System.assert(false,'interface required');
  } catch(TypeException expected) {}
 }
}
