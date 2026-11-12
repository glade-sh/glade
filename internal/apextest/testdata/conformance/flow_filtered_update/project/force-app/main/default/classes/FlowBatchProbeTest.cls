@IsTest
private class FlowBatchProbeTest {
    @IsTest static void observe() {
        User admin=[SELECT ProfileId FROM User WHERE Id=:UserInfo.getUserId()];
        User subject=new User(Alias='fbbs',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=admin.ProfileId,TimeZoneSidKey='America/Los_Angeles',Username='flowbatchbatch.'+Datetime.now().getTime()+'@example.invalid');
        insert subject;
        PermissionSet access=[SELECT Id FROM PermissionSet WHERE Name='FlowCoreAOCaptureAccess' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId=subject.Id,PermissionSetId=access.Id);
        Map<String,Object> out=new Map<String,Object>();
        System.runAs(subject) {
            for (Integer n:new List<Integer>{2}) {
                FlowCoreAOAddDays.sizes=new List<Integer>();
                List<FlowCoreAORecord__c> rows=new List<FlowCoreAORecord__c>();
                for (Integer i=0;i<n;i++) rows.add(new FlowCoreAORecord__c(Name='Synthetic '+i,Unit__c='value',Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0)));
                try { insert rows; out.put(String.valueOf(n),new Map<String,Object>{'success'=>true,'sizes'=>FlowCoreAOAddDays.sizes,'inserted'=>[SELECT count() FROM FlowCoreAORecord__c WHERE Id IN :rows AND Unit__c = 'updated']}); }
                catch (Exception e) { out.put(String.valueOf(n),new Map<String,Object>{'success'=>false,'error'=>e.getTypeName()+': '+e.getMessage()}); }
            }
        }
        System.assert(false,'FLOWFILTER|'+JSON.serialize(out));
    }
}
