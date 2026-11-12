@IsTest
private class FlowBatchProbeTest {
    @IsTest static void observe() {
        User admin=[SELECT ProfileId FROM User WHERE Id=:UserInfo.getUserId()];
        User subject=new User(Alias='fbrf',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=admin.ProfileId,TimeZoneSidKey='America/Los_Angeles',Username='flowbatchrf.'+Datetime.now().getTime()+'@example.invalid');
        insert subject;
        PermissionSet access=[SELECT Id FROM PermissionSet WHERE Name='FlowCoreAOCaptureAccess' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId=subject.Id,PermissionSetId=access.Id);
        Map<String,Object> out=new Map<String,Object>();
        System.runAs(subject) {
            for (Integer n:new List<Integer>{2,3}) {
                FlowCoreAOAddDays.sizes=new List<Integer>();
                FlowBatchRefireLog.log=new List<String>();
                List<FlowCoreAORecord__c> rows=new List<FlowCoreAORecord__c>();
                for (Integer i=0;i<n;i++) rows.add(new FlowCoreAORecord__c(Name='Synthetic '+i,Unit__c='value',Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0)));
                Map<String,Object> shape=new Map<String,Object>();
                try {
                    insert rows;
                    shape.put('success',true);
                    shape.put('sizes',FlowCoreAOAddDays.sizes);
                    shape.put('triggerLog',FlowBatchRefireLog.log);
                    List<Object> units=new List<Object>();
                    for (FlowCoreAORecord__c r:[SELECT Name,Unit__c,DueDate__c FROM FlowCoreAORecord__c WHERE Id IN :rows ORDER BY Id]) units.add(new Map<String,Object>{'name'=>r.Name,'unit'=>r.Unit__c,'date'=>r.DueDate__c});
                    shape.put('rows',units);
                } catch (Exception e) { shape.put('success',false); shape.put('error',e.getTypeName()+': '+e.getMessage()); }
                out.put(String.valueOf(n),shape);
            }
        }
        System.assert(false,'FLOWREFIRE|'+JSON.serialize(out));
    }
}
