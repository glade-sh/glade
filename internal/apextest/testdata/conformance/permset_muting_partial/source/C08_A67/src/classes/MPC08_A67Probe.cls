@isTest
private class MPC08_A67Probe {
    // No expected permission outcome is asserted. Final failure transports observations
    // without requiring debug logs or leaving users/assignments behind after test rollback.
    @isTest static void capture() {
        Map<String,Object> out = new Map<String,Object>{'schema'=>'muting-partial-deps-v1','fixtureId'=>'C08_A67','transportOnly'=>true};
        try {
            Profile standard = [SELECT Id, Name, UserLicense.Name FROM Profile WHERE Name='Standard User' AND UserLicense.Name='Salesforce'];
            out.put('implicitProfile',new Map<String,Object>{'id'=>standard.Id,'name'=>standard.Name,'license'=>standard.UserLicense.Name});
            PermissionSet profileSet=[SELECT Id,Name,IsOwnedByProfile FROM PermissionSet WHERE ProfileId=:standard.Id AND IsOwnedByProfile=true];
            out.put('implicitProfilePermissionSetId',profileSet.Id);
            List<ObjectPermissions> profilePerms=[SELECT SobjectType,PermissionsCreate,PermissionsRead,PermissionsEdit,PermissionsDelete,PermissionsViewAllRecords,PermissionsModifyAllRecords FROM ObjectPermissions WHERE ParentId=:profileSet.Id AND SobjectType='MPC08_A67__c'];
            List<Object> profileRows=new List<Object>();
            for(ObjectPermissions p:profilePerms){profileRows.add(new Map<String,Object>{'object'=>p.SobjectType,'create'=>p.PermissionsCreate,'read'=>p.PermissionsRead,'edit'=>p.PermissionsEdit,'delete'=>p.PermissionsDelete,'viewAll'=>p.PermissionsViewAllRecords,'modifyAll'=>p.PermissionsModifyAllRecords});}
            out.put('implicitProfileObjectPermissionRows',profileRows);
            PermissionSetGroup grp = [SELECT Id, DeveloperName, Status FROM PermissionSetGroup WHERE DeveloperName='MPC08_A67Group'];
            out.put('groupId',grp.Id);out.put('groupStatusBefore',grp.Status);
            if (grp.Status != 'Updated') {
                // One explicit calculation in rollback-only test; no assignment retry.
                Test.calculatePermissionSetGroup(grp.Id);
            }
            grp=[SELECT Id, Status FROM PermissionSetGroup WHERE Id=:grp.Id];
            out.put('groupStatusAfter',grp.Status);
            String nonce=EncodingUtil.convertToHex(Crypto.generateAesKey(128));
            User actor=new User(ProfileId=standard.Id,Alias='mpUser',LastName='Synthetic Muting',Username='mp-C08_A67-'+UserInfo.getOrganizationId()+'-'+nonce+'@example.invalid',Email='mp@example.invalid',TimeZoneSidKey='GMT',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',UserRoleId=null);
            insert actor;
            insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetGroupId=grp.Id);
            List<PermissionSetAssignment> assignments=[SELECT PermissionSetId,PermissionSetGroupId,PermissionSet.IsOwnedByProfile FROM PermissionSetAssignment WHERE AssigneeId=:actor.Id LIMIT 21];
            Integer groupCount=0;Integer directCount=0;Integer otherGroupCount=0;
            List<Object> assignmentRows=new List<Object>();
            for(PermissionSetAssignment a:assignments){
                assignmentRows.add(new Map<String,Object>{'permissionSetId'=>a.PermissionSetId,'permissionSetGroupId'=>a.PermissionSetGroupId,'isOwnedByProfile'=>a.PermissionSet.IsOwnedByProfile});
                if(a.PermissionSetGroupId==grp.Id){groupCount++;}
                else if(a.PermissionSetGroupId!=null){otherGroupCount++;}
                else if(a.PermissionSetId!=null && !a.PermissionSet.IsOwnedByProfile){directCount++;}
            }
            out.put('assignmentRows',assignmentRows);
            out.put('assignedGroupCount',groupCount);out.put('directPermissionSetCount',directCount);out.put('otherGroupCount',otherGroupCount);
            out.put('assignmentIsolationObserved',groupCount==1 && directCount==0 && otherGroupCount==0);
            out.put('profileIsImplicitAndNotMuted',true);
            System.runAs(actor){
                Schema.DescribeSObjectResult d=Schema.getGlobalDescribe().get('MPC08_A67__c').getDescribe();
                out.put('isUpdateable',d.isUpdateable());out.put('isDeletable',d.isDeletable());
                out.put('actorProfileId',UserInfo.getProfileId());
            }
            out.put('observationComplete',true);
        }catch(Exception ex){
            out.put('observationComplete',false);out.put('exceptionType',ex.getTypeName());out.put('exceptionMessage',ex.getMessage());
        }
        // Counters precede final encoding; they are not whole-execution heap/CPU claims.
        out.put('limits',new Map<String,Object>{'queries'=>Limits.getQueries(),'dmlStatements'=>Limits.getDmlStatements(),'dmlRows'=>Limits.getDmlRows(),'cpuMs'=>Limits.getCpuTime(),'heapBytes'=>Limits.getHeapSize()});
        System.assert(false,'MP_CAPTURE_V1:'+EncodingUtil.base64Encode(Blob.valueOf(JSON.serialize(out))));
    }
}
