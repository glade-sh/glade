public without sharing class SynthFlsProbe67 {
    private static final String SOURCE_API='67.0';
    private static List<String> payload;
    private static Map<String,Object> submission;
    private static Set<Id> restrictedRecordIds;
    private static Id restrictedUserId;
    private static String enc(String value) { return EncodingUtil.base64Encode(Blob.valueOf(value)); }
    private static void row(Map<String,Object> value) { payload.add(JSON.serialize(value)); }
    private static Map<String,Object> exceptionData(Exception ex) {
        Map<String,Object> value=new Map<String,Object>{'typeName'=>ex.getTypeName(),'message'=>ex.getMessage(),'stackTrace'=>ex.getStackTraceString()};
        if(ex instanceof DmlException) {
            DmlException dx=(DmlException)ex;List<Object> details=new List<Object>();
            for(Integer i=0;i<dx.getNumDml();i++) details.add(new Map<String,Object>{'ordinal'=>i,'getDmlIndex'=>dx.getDmlIndex(i),'getDmlId'=>dx.getDmlId(i),'getDmlMessage'=>dx.getDmlMessage(i),'getDmlType'=>dx.getDmlType(i),'getDmlFieldNames'=>dx.getDmlFieldNames(i)});
            value.put('getNumDml',dx.getNumDml());value.put('rows',details);
        }
        return value;
    }
    public static void beginCapture(String route) {
        payload=new List<String>();
        submission=new Map<String,Object>{'organizationId'=>UserInfo.getOrganizationId(),'userId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId(),'sourceApi'=>SOURCE_API,'route'=>route};
        Map<String,Object> context=new Map<String,Object>();context.putAll(submission);
        context.put('kind','context');context.put('isTest',Test.isRunningTest());context.put('locale',UserInfo.getLocale());context.put('timezone',UserInfo.getTimeZone().getID());context.put('language',UserInfo.getLanguage());
        row(context);
    }
    private static Map<String,Object> describeObject(Schema.SObjectType token,List<String> names) {
        Schema.DescribeSObjectResult obj=token.getDescribe();Map<String,Schema.SObjectField> nativeFields=obj.fields.getMap();Map<String,Object> fields=new Map<String,Object>();
        for(String name:names) {
            if(!nativeFields.containsKey(name)) {fields.put(name,new Map<String,Object>{'present'=>false});continue;}
            Schema.DescribeFieldResult field=nativeFields.get(name).getDescribe();
            fields.put(name,new Map<String,Object>{'present'=>true,'accessible'=>field.isAccessible(),'createable'=>field.isCreateable(),'updateable'=>field.isUpdateable(),'permissionable'=>field.isPermissionable(),'nillable'=>field.isNillable()});
        }
        return new Map<String,Object>{'accessible'=>obj.isAccessible(),'createable'=>obj.isCreateable(),'updateable'=>obj.isUpdateable(),'fields'=>fields};
    }
    public static void auditActor(String actorLabel,Id uid,Id pid) {
        List<PermissionSet> parents=[SELECT Id,Name,IsOwnedByProfile,ProfileId,Profile.Name,PermissionsViewAllData,PermissionsModifyAllData,PermissionsViewSetup FROM PermissionSet WHERE ProfileId=:pid WITH SYSTEM_MODE ORDER BY Id];
        List<PermissionSetAssignment> assignments=[SELECT Id,AssigneeId,PermissionSetId,PermissionSet.Name,PermissionSetGroupId FROM PermissionSetAssignment WHERE AssigneeId=:uid WITH SYSTEM_MODE ORDER BY Id];
        Set<Id> parentIds=new Set<Id>();for(PermissionSet ps:parents)parentIds.add(ps.Id);for(PermissionSetAssignment ps:assignments)parentIds.add(ps.PermissionSetId);
        List<FieldPermissions> fp=[SELECT Id,ParentId,SobjectType,Field,PermissionsRead,PermissionsEdit FROM FieldPermissions WHERE ParentId IN :parentIds AND Field IN ('Case.OwnerId','Case.Subject','Case.Status','Case.Origin','SynthAccess__c.ReadField__c','SynthAccess__c.HiddenField__c','User.Id','User.Name','User.Email','User.SynthUserField__c') WITH SYSTEM_MODE ORDER BY ParentId,SobjectType,Field,Id];
        List<ObjectPermissions> op=[SELECT Id,ParentId,SobjectType,PermissionsRead,PermissionsCreate,PermissionsEdit,PermissionsDelete,PermissionsViewAllRecords,PermissionsModifyAllRecords FROM ObjectPermissions WHERE ParentId IN :parentIds AND SobjectType IN ('Case','SynthAccess__c','User') WITH SYSTEM_MODE ORDER BY ParentId,SobjectType,Id];
        List<User> users=[SELECT Id,ProfileId,Profile.Name,Profile.UserLicense.Name,UserRoleId,UserType,IsActive FROM User WHERE Id=:uid WITH SYSTEM_MODE];
        List<PermissionSet> assignedSets=[SELECT Id,Name,PermissionsViewAllData,PermissionsModifyAllData,PermissionsViewSetup FROM PermissionSet WHERE Id IN :parentIds WITH SYSTEM_MODE ORDER BY Id];
        row(new Map<String,Object>{'kind'=>'permissionsEvidence','actor'=>actorLabel,'userId'=>uid,'profileId'=>pid,'observerUserId'=>UserInfo.getUserId(),'users'=>users,'assignedSets'=>assignedSets,'profileParents'=>parents,'assignments'=>assignments,'fieldPermissions'=>fp,'objectPermissions'=>op});
    }
    private static void permissions(String actorLabel) {
        Map<String,Object> described=new Map<String,Object>{'Case'=>describeObject(Case.SObjectType,new List<String>{'OwnerId','Subject','Status','Origin'}),'SynthAccess__c'=>describeObject(SynthAccess__c.SObjectType,new List<String>{'ReadField__c','HiddenField__c'}),'User'=>describeObject(User.SObjectType,new List<String>{'Id','Name','Email','SynthUserField__c'})};
        row(new Map<String,Object>{'kind'=>'permissions','actor'=>actorLabel,'userId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId(),'describe'=>described});
    }
    private static void stripProbe(String actorLabel,String probeId,System.AccessType access,List<SObject> records) {
        Map<String,Object> result=new Map<String,Object>{'kind'=>'probe','actor'=>actorLabel,'probeId'=>probeId,'input'=>JSON.deserializeUntyped(JSON.serialize(records)),'enforceRootObjectCRUD'=>true};
        try {
            SObjectAccessDecision decision=Security.stripInaccessible(access,records,true);
            List<Object> maps=new List<Object>();for(SObject record:decision.getRecords())maps.add(record.getPopulatedFieldsAsMap());
            result.put('outcome','success');result.put('getRemovedFields',decision.getRemovedFields());result.put('recordMaps',maps);result.put('records',decision.getRecords());
        } catch(Exception ex) {result.put('outcome','exception');result.put('exception',exceptionData(ex));}
        row(result);
    }
    public static void captureActor(String actorLabel,Id otherUserId,Boolean fullMatrix) {
        Id actorId=UserInfo.getUserId();
        row(new Map<String,Object>{'kind'=>'actor','actor'=>actorLabel,'userId'=>actorId,'profileId'=>UserInfo.getProfileId(),'isTest'=>Test.isRunningTest(),'otherUserId'=>otherUserId,'fullMatrix'=>fullMatrix});
        permissions(actorLabel);
        // Every setup DML is explicitly SYSTEM_MODE. Distinct seeds avoid one operation masking another.
        List<Case> seeds=new List<Case>{new Case(Subject='Synthetic select',Status='New',Origin='Web',OwnerId=actorId),new Case(Subject='Synthetic update system',Status='New',Origin='Web',OwnerId=actorId),new Case(Subject='Synthetic update user',Status='New',Origin='Web',OwnerId=actorId)};
        Database.insert(seeds,true,AccessLevel.SYSTEM_MODE);
        for(String mode:new List<String>{'system','user'}) {
            Case inserted=new Case(Subject='Synthetic insert '+mode,Status='New',Origin='Web',OwnerId=actorId);
            Map<String,Object> result=new Map<String,Object>{'kind'=>'probe','actor'=>actorLabel,'probeId'=>'case.insert.'+mode,'input'=>JSON.deserializeUntyped(JSON.serialize(inserted))};
            try {
                if(mode=='system') {insert as system inserted;} else {insert as user inserted;}
                result.put('outcome','success');result.put('recordId',inserted.Id);
            } catch(Exception ex) {result.put('outcome','exception');result.put('exception',exceptionData(ex));}
            row(result);
            Id targetId=mode=='system'?seeds[1].Id:seeds[2].Id;
            Case changed=new Case(Id=targetId,OwnerId=otherUserId==null?actorId:otherUserId);
            result=new Map<String,Object>{'kind'=>'probe','actor'=>actorLabel,'probeId'=>'case.update.'+mode,'input'=>JSON.deserializeUntyped(JSON.serialize(changed)),'changesOwner'=>otherUserId!=null};
            try {
                if(mode=='system') {update as system changed;} else {update as user changed;}
                result.put('outcome','success');
            } catch(Exception ex) {result.put('outcome','exception');result.put('exception',exceptionData(ex));}
            row(result);
            Id selectedId=seeds[0].Id;
            result=new Map<String,Object>{'kind'=>'probe','actor'=>actorLabel,'probeId'=>'case.select.'+mode,'recordId'=>selectedId};
            try {
                List<Case> found;
                if(mode=='system') found=[SELECT Id,OwnerId FROM Case WHERE Id=:selectedId WITH SYSTEM_MODE];
                else found=[SELECT Id,OwnerId FROM Case WHERE Id=:selectedId WITH USER_MODE];
                result.put('outcome','success');result.put('records',found);
            } catch(Exception ex) {result.put('outcome','exception');result.put('exception',exceptionData(ex));}
            row(result);
        }
        stripProbe(actorLabel,'case.strip.updatable',System.AccessType.UPDATABLE,new List<SObject>{new Case(Id=seeds[0].Id,OwnerId=otherUserId==null?actorId:otherUserId)});
        if(!fullMatrix)return;
        for(System.AccessType access:new List<System.AccessType>{System.AccessType.READABLE,System.AccessType.CREATABLE,System.AccessType.UPDATABLE}) {
            stripProbe(actorLabel,'custom.strip.'+String.valueOf(access).toLowerCase(),access,new List<SObject>{new SynthAccess__c(ReadField__c='synthetic-readable',HiddenField__c='synthetic-hidden')});
        }
        List<Id> targets=new List<Id>{actorId};if(otherUserId!=null)targets.add(otherUserId);
        for(Integer targetIndex=0;targetIndex<targets.size();targetIndex++) for(String mode:new List<String>{'system','user'}) for(String shape:new List<String>{'standard','custom'}) {
            Id targetUserId=targets[targetIndex];
            String relation=targetIndex==0?'own':'other';
            Map<String,Object> result=new Map<String,Object>{'kind'=>'probe','actor'=>actorLabel,'probeId'=>'user.select.'+relation+'.'+shape+'.'+mode,'targetUserId'=>targetUserId};
            try {
                List<User> found;
                if(shape=='custom') {
                    if(mode=='system')found=[SELECT Id,Name,Email,SynthUserField__c FROM User WHERE Id=:targetUserId WITH SYSTEM_MODE];
                    else found=[SELECT Id,Name,Email,SynthUserField__c FROM User WHERE Id=:targetUserId WITH USER_MODE];
                } else {
                    if(mode=='system')found=[SELECT Id,Name,Email FROM User WHERE Id=:targetUserId WITH SYSTEM_MODE];
                    else found=[SELECT Id,Name,Email FROM User WHERE Id=:targetUserId WITH USER_MODE];
                }
                result.put('outcome','success');result.put('records',found);
            } catch(Exception ex) {result.put('outcome','exception');result.put('exception',exceptionData(ex));}
            row(result);
        }
        List<SynthAccess__c> owned=new List<SynthAccess__c>{new SynthAccess__c(Name='Synthetic own',OwnerId=actorId)};
        if(otherUserId!=null)owned.add(new SynthAccess__c(Name='Synthetic other',OwnerId=otherUserId));
        Database.insert(owned,true,AccessLevel.SYSTEM_MODE);Set<Id> recordIds=new Set<Id>();for(SynthAccess__c record:owned)recordIds.add(record.Id);
        Map<String,Object> result=new Map<String,Object>{'kind'=>'probe','actor'=>actorLabel,'probeId'=>'recordAccess','inputOwners'=>owned,'queriedUserId'=>actorId,'queryMode'=>'SYSTEM_MODE','otherOwnerAvailable'=>otherUserId!=null};
        try {
            List<UserRecordAccess> found=[SELECT RecordId,HasReadAccess,HasEditAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:actorId AND RecordId IN :recordIds WITH SYSTEM_MODE];
            result.put('outcome','success');result.put('records',found);
        } catch(Exception ex) {result.put('outcome','exception');result.put('exception',exceptionData(ex));}
        row(result);
        if(actorLabel=='minimal'){restrictedRecordIds=recordIds;restrictedUserId=actorId;}
    }
    public static void adminRecordAccess() {
        Map<String,Object> result=new Map<String,Object>{'kind'=>'probe','actor'=>'admin-diagnostic','probeId'=>'recordAccess.admin','observerUserId'=>UserInfo.getUserId(),'queriedUserId'=>restrictedUserId,'recordIds'=>restrictedRecordIds,'queryMode'=>'SYSTEM_MODE'};
        try {
            List<UserRecordAccess> found=[SELECT RecordId,HasReadAccess,HasEditAccess,MaxAccessLevel FROM UserRecordAccess WHERE UserId=:restrictedUserId AND RecordId IN :restrictedRecordIds WITH SYSTEM_MODE];
            result.put('outcome','success');result.put('records',found);
        } catch(Exception ex) {result.put('outcome','exception');result.put('exception',exceptionData(ex));}
        row(result);
    }
    public static String finishCapture() {
        row(new Map<String,Object>{'kind'=>'limits','dmlStatements'=>Limits.getDmlStatements(),'dmlRows'=>Limits.getDmlRows(),'queries'=>Limits.getQueries(),'cpuMs'=>Limits.getCpuTime(),'heapBytes'=>Limits.getHeapSize()});
        Integer total=0;for(String value:payload){Integer count=enc(value).length();if(count>12000)throw new IllegalArgumentException('SYNTH_FLS_ROW_LIMIT');total+=count+100;}
        if(total>700000)throw new IllegalArgumentException('SYNTH_FLS_TRANSACTION_LIMIT');
        for(Integer i=0;i<payload.size();i++)System.debug(LoggingLevel.ERROR,'SYNTH_FLS_ROW|'+i+'|'+enc(payload[i]));
        Map<String,Object> terminal=new Map<String,Object>();terminal.putAll(submission);terminal.put('payloadRows',payload.size());terminal.put('sha256',EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',Blob.valueOf(String.join(payload,'\n')))));
        String carrier='SYNTH_FLS_END|'+enc(JSON.serialize(terminal));System.debug(LoggingLevel.ERROR,carrier);return carrier;
    }
}
