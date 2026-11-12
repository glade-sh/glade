@IsTest private class NullProbe67_00 { @IsTest static void m00(){
 Schema.SObjectType it=Schema.getGlobalDescribe().get('Individual');Schema.SObjectType st=Schema.getGlobalDescribe().get('Seller');SObject individual=NullCore67.build(it,1,null);insert individual;
 SObject missing=NullCore67.build(st,1,'PartyId');SObject control=NullCore67.build(st,1,'PartyId');control.put('PartyId',individual.Id);
 Database.SaveResult a=Database.insert(missing,false);Database.SaveResult b=Database.insert(control,false);
 System.assert(false,'CAPTURE|Seller|'+JSON.serialize(new Map<String,Object>{'PartyId'=>new Map<String,Object>{'nullSuccess'=>a.isSuccess(),'nullErrors'=>a.getErrors(),'controlSuccess'=>b.isSuccess(),'controlErrors'=>b.getErrors()}}));
} }