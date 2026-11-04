@IsTest private class Case251Query67 { @IsTest static void observe() {
Lead lead = new Lead(LastName='Route', Company='Synthetic');
insert lead;
Lead result = [SELECT Id, LastTransferDate FROM Lead WHERE Id = :lead.Id];
Boolean rejected = false;
try { Database.query('SELECT Case251DefinitelyMissing__c FROM Lead LIMIT 1'); } catch (QueryException e) { rejected = true; }
String output = 'query=' + String.valueOf(result.Id == lead.Id) + '|boundary=' + String.valueOf(Schema.SObjectType.Lead.fields.getMap().containsKey('Id')) + '|rejection=' + String.valueOf(rejected) + '|type=' + String.valueOf(Schema.SObjectType.Lead.fields.getMap().get('LastTransferDate').getDescribe().getType());
System.assert(false, 'CASE251|' + output);
} }
