@IsTest
private class FlowBatchProbeTest {
    @IsTest static void observe() {
        User admin=[SELECT ProfileId FROM User WHERE Id=:UserInfo.getUserId()];
        User subject=new User(Alias='fbfu',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=admin.ProfileId,TimeZoneSidKey='America/Los_Angeles',Username='flowbatchfu.'+Datetime.now().getTime()+'@example.invalid');
        insert subject;
        PermissionSet access=[SELECT Id FROM PermissionSet WHERE Name='FlowCoreAOCaptureAccess' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId=subject.Id,PermissionSetId=access.Id);
        Map<String,Object> out=new Map<String,Object>();
        System.runAs(subject) {
            Map<String,List<String>> shapes=new Map<String,List<String>>{'allfault'=>new List<String>{'throw','throw'},'middle'=>new List<String>{'value','throw','value'},'nofault'=>new List<String>{'value','value'}};
            for (String key:shapes.keySet()) {
                FlowCoreAOAddDays.sizes=new List<Integer>();
                List<FlowCoreAORecord__c> rows=new List<FlowCoreAORecord__c>();
                Integer i=0;
                for (String mode:shapes.get(key)) rows.add(new FlowCoreAORecord__c(Name='Synthetic '+(i++),Unit__c=mode,Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0)));
                Map<String,Object> shape=new Map<String,Object>();
                try {
                    insert rows;
                    shape.put('success',true);
                    shape.put('sizes',FlowCoreAOAddDays.sizes);
                    List<Object> units=new List<Object>();
                    for (FlowCoreAORecord__c r:[SELECT Name,Unit__c,DueDate__c FROM FlowCoreAORecord__c WHERE Id IN :rows ORDER BY Name]) units.add(new Map<String,Object>{'name'=>r.Name,'unit'=>r.Unit__c,'date'=>r.DueDate__c});
                    shape.put('rows',units);
                } catch (Exception e) { shape.put('success',false); shape.put('error',e.getTypeName()+': '+e.getMessage()); }
                out.put(key,shape);
            }
        }
        System.assert(false,'FLOWFAULTUPDATE|'+JSON.serialize(out));
    }
}
