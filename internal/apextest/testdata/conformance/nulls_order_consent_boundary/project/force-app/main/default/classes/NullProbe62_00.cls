@IsTest private class NullProbe62_00 { @IsTest static void m00(){
 Map<String,Schema.SObjectType> g=Schema.getGlobalDescribe();SObject text=NullCore62.build(g.get('AuthorizationFormText'),1,null);insert text;
 SObject missing=NullCore62.build(g.get('AuthorizationFormConsent'),1,'AuthorizationFormTextId');SObject control=NullCore62.build(g.get('AuthorizationFormConsent'),1,'AuthorizationFormTextId');control.put('AuthorizationFormTextId',text.Id);
 Database.SaveResult a=Database.insert(missing,false);Database.SaveResult b=Database.insert(control,false);
 System.assert(false,'CAPTURE|AuthorizationFormConsent|'+JSON.serialize(new Map<String,Object>{'AuthorizationFormTextId'=>new Map<String,Object>{'nullSuccess'=>a.isSuccess(),'nullErrors'=>a.getErrors(),'controlSuccess'=>b.isSuccess(),'controlErrors'=>b.getErrors()}}));
} }