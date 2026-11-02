@IsTest
private class FlowBatchProbeTest {
    @IsTest static void observe() {
        User admin=[SELECT ProfileId FROM User WHERE Id=:UserInfo.getUserId()];
        User pacific=new User(Alias='fbtz',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=admin.ProfileId,TimeZoneSidKey='America/Los_Angeles',Username='flowbatchtz.'+DateTime.now().getTime()+'@example.invalid');
        insert pacific;
        PermissionSet access=[SELECT Id FROM PermissionSet WHERE Name='FlowCoreAOCaptureAccess' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId=pacific.Id,PermissionSetId=access.Id);
        Map<String,Object> out=new Map<String,Object>();
        System.runAs(pacific) {
            List<FlowCoreAORecord__c> samples=new List<FlowCoreAORecord__c>{
                new FlowCoreAORecord__c(Name='first',Unit__c='value',Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0)),
                new FlowCoreAORecord__c(Name='null',Unit__c='null-element',Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0),DueDate__c=Date.newInstance(2020,1,2),DueDateTime__c=Datetime.newInstanceGmt(2020,1,2,3,4,5))
            };
            try {
                insert samples;
                List<Object> values=new List<Object>();
                for (FlowCoreAORecord__c r:[SELECT Name,DueDate__c,DueDateTime__c FROM FlowCoreAORecord__c WHERE Id IN :samples ORDER BY Name]) values.add(new Map<String,Object>{'name'=>r.Name,'date'=>r.DueDate__c,'datetime'=>r.DueDateTime__c});
                out.put('dates',values);
            } catch (DmlException e) { out.put('datesError',e.getDmlMessage(0)); }
            List<FlowCoreAORecord__c> faulty=new List<FlowCoreAORecord__c>{
                new FlowCoreAORecord__c(Name='first',Unit__c='skip',Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0)),
                new FlowCoreAORecord__c(Name='middle',Unit__c='throw',Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0)),
                new FlowCoreAORecord__c(Name='last',Unit__c='value',Start__c=Datetime.newInstanceGmt(2031,3,10,0,30,0))
            };
            try { insert faulty; out.put('fault','none'); }
            catch (DmlException e) {
                List<Object> errors=new List<Object>();
                for(Integer i=0;i<e.getNumDml();i++) errors.add(new Map<String,Object>{'index'=>e.getDmlIndex(i),'status'=>String.valueOf(e.getDmlStatusCode(i)),'containsFault'=>e.getDmlMessage(i).contains('synthetic action fault')});
                out.put('numDml',e.getNumDml());out.put('errors',errors);
            }
        }
        System.assert(false,'FLOWSUBSET|'+JSON.serialize(out));
    }
}
