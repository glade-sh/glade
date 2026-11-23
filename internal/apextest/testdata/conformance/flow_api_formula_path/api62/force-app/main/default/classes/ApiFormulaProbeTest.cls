@IsTest private class ApiFormulaProbeTest {
 @IsTest static void probe() {
  Contact c=new Contact(LastName='Api Formula Probe'); insert c;
  String observed=[SELECT Description FROM Contact WHERE Id=:c.Id].Description;
  System.assert(false,'OBS=scheme='+observed.startsWith('https://')+'|substituted='+observed.endsWith('.example.test/probe/path'));
 }
}
