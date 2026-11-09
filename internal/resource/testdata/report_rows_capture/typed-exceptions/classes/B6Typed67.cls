@IsTest private class B6Typed67 {
 @IsTest static void check(){
  Reports.InvalidFilterException a = new Reports.InvalidFilterException('Synthetic invalid');
  Reports.MetadataException b = null;
  Reports.ReportRunException c = null;
  if (b != null) System.assertNotEquals(null, b.getMessage());
  if (c != null) System.assertNotEquals(null, c.getMessage());
  System.assertEquals('Synthetic invalid', a.getMessage());
  System.assertEquals('reports.InvalidFilterException', a.getTypeName());
 }
}
