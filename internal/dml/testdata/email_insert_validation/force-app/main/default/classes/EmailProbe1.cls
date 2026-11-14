@IsTest private class EmailProbe1 { @IsTest static void runProbe() {
List<String> vals = new List<String>{'ada@orbel.example.','ada_1@orbel.example','ada1@orbel.example','ada2@orbel.example','test@test.example','test1@test.example'};
List<String> out = new List<String>();
for (Integer i=0; i<vals.size(); i++) {
 for (Integer k=0; k<3; k++) {
  for (Integer mode=0; mode<2; mode++) {
   SObject rec;
   if (k==1) rec = new Lead(LastName='Probe', Company='Fixture');
   else rec = new Contact(LastName='Probe');
   String fieldName = k==2 ? 'ProbeEmail__c' : 'Email';
   rec.put(fieldName, vals[i]);
   String result = 'OK';
   if (mode==0) {
    Database.SaveResult sr=Database.insert(rec,false);
    if (!sr.isSuccess()) { Database.Error e=sr.getErrors()[0]; result=String.valueOf(e.getStatusCode())+'~'+e.getMessage()+'~'+String.valueOf(e.getFields()); }
   } else {
    try { insert rec; } catch (DmlException e) { result=String.valueOf(e.getDmlStatusCode(0))+'~'+e.getDmlMessage(0)+'~'+String.valueOf(e.getDmlFields(0)); }
   }
   if (result=='OK') delete rec;
   out.add(String.valueOf(i+6)+':'+String.valueOf(k)+':'+String.valueOf(mode)+':'+result);
  }
 }
}
String payload=String.join(out,'|');
System.assert(false,'OBSBEGIN'+payload+'OBSEND');
}}