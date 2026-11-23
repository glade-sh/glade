@IsTest private class ApiEmailProbeTest {
 @IsTest static void probe() {
  Contact c=new Contact(LastName='Api Email Probe',Email='person@example.test'); insert c; c.Title='Updated'; update c;
  String observed=[SELECT Description FROM Contact WHERE LastName='Api Email Marker' LIMIT 1].Description;
  Boolean replaced=observed != null && observed.contains('.example.test/probe/path');
  observed=observed == null ? '<null>' : observed.replaceAll('https?://[^/|]+','<host>').replaceAll('00D[a-zA-Z0-9]{12,15}','<org>');
  System.assert(false, 'OBS='+observed+'|substituted='+replaced);
 }
}
