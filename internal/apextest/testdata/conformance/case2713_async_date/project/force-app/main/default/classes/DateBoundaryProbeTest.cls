@IsTest private class DateBoundaryProbeTest {
 @IsTest static void observe() {
  List<String> inputs=new List<String>{'1990-01-01','1/2/1990','2/29/2024','2/29/2023','01/02/1990'};
  List<String> values=new List<String>(); for(String s:inputs) values.add(DateBoundaryProbe.observe(s));
  System.assert(false,'DATE_BOUNDARY|'+JSON.serialize(values));
 }
}
