public class HierarchyNameOwnerObserver {
    public static void seed(String tag, String ownerMode, String nameMode) {
        Map<String,Object> payload = measure(ownerMode, nameMode);
        insert new Account(Name='HNAME '+tag, Description=JSON.serialize(payload));
    }

    public static String part(String tag, Integer partNumber) {
        String key = 'HNAME '+tag;
        List<Account> saved = [SELECT Description FROM Account WHERE Name=:key LIMIT 2];
        System.assertEquals(1, saved.size(), 'HNAME transport row count');
        String raw = saved[0].Description;
        String encoded = EncodingUtil.base64Encode(Blob.valueOf(raw));
        System.assert(encoded.length() <= 10240, 'HNAME transport overflow');
        String hash = EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(raw)));
        Integer offset = partNumber*320;
        String chunk = encoded.length()>offset ? encoded.substring(offset, Math.min(offset+320,encoded.length())) : '';
        String envelope = JSON.serialize(new Map<String,Object>{'part'=>partNumber,'total'=>32,'hash'=>hash,'chunk'=>chunk});
        System.assert(envelope.length()+tag.length()+56<=620, 'HNAME envelope overflow');
        return 'HNAME_RESULT|'+tag+'|'+envelope;
    }

    public static Map<String,Object> measure(String ownerMode, String nameMode) {
        Map<String,Object> out = new Map<String,Object>{'ownerMode'=>ownerMode,'nameMode'=>nameMode,'stage'=>'input','context'=>new Map<String,Object>{'organizationId'=>UserInfo.getOrganizationId(),'userId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId()}};
        OwnerNamePreference__c input = new OwnerNamePreference__c();
        Id savedId;
        try {
            if(ownerMode=='organization') input.SetupOwnerId=UserInfo.getOrganizationId();
            if(ownerMode=='user') input.SetupOwnerId=UserInfo.getUserId();
            if(ownerMode=='profile') input.SetupOwnerId=UserInfo.getProfileId();
            if(nameMode=='explicit') input.Name='Assigned marker';
            if(nameMode=='null') input.Name=null;
            if(nameMode=='empty') input.Name='';
            out.put('input', snapshot(input));
            out.put('ownerWasSupplied',ownerMode!='omitted');
            out.put('nameWasSupplied',nameMode!='omitted');
            out.put('label',OwnerNamePreference__c.SObjectType.getDescribe().getLabel());
            out.put('stage','dml');
            Database.SaveResult result=Database.insert(input,false);
            List<Object> errors=new List<Object>();
            for(Database.Error error:result.getErrors()) errors.add(new Map<String,Object>{'status'=>String.valueOf(error.getStatusCode()),'message'=>error.getMessage(),'fields'=>error.getFields()});
            out.put('dml',new Map<String,Object>{'success'=>result.isSuccess(),'errors'=>errors});
            savedId=result.getId();
            out.put('insertedId',savedId);
            out.put('callerAfterDml',snapshot(input));
        } catch(Exception error) {
            out.put('measurementError',fault(error));
        }
        for(String view:new List<String>{'soql','orgDefaults','instance','instanceOrganization','instanceUser','instanceProfile'}) {
            try {
                OwnerNamePreference__c record;
                if(view=='soql' && savedId!=null) record=[SELECT Name,SetupOwnerId FROM OwnerNamePreference__c WHERE Id=:savedId];
                if(view=='orgDefaults') record=OwnerNamePreference__c.getOrgDefaults();
                if(view=='instance') record=OwnerNamePreference__c.getInstance();
                if(view=='instanceOrganization') record=OwnerNamePreference__c.getInstance(UserInfo.getOrganizationId());
                if(view=='instanceUser') record=OwnerNamePreference__c.getInstance(UserInfo.getUserId());
                if(view=='instanceProfile') record=OwnerNamePreference__c.getInstance(UserInfo.getProfileId());
                out.put(view,snapshot(record));
            } catch(Exception error) {
                out.put(view,fault(error));
            }
        }
        out.put('stage','complete');
        return out;
    }

    private static Map<String,Object> snapshot(OwnerNamePreference__c record) {
        Map<String,Object> result=new Map<String,Object>{'present'=>record!=null,'Name'=>null,'SetupOwnerId'=>null,'ownerIsOrganization'=>null,'ownerIsUser'=>null,'ownerIsProfile'=>null,'nameEqualsOwnerString'=>null,'nameEqualsOrganizationString'=>null,'nameEqualsUserString'=>null,'nameEqualsProfileString'=>null};
        if(record!=null) {
            String name=record.Name;
            Id owner=record.SetupOwnerId;
            result.put('Name',name);
            result.put('SetupOwnerId',owner);
            result.put('ownerIsOrganization',owner==null?null:owner==UserInfo.getOrganizationId());
            result.put('ownerIsUser',owner==null?null:owner==UserInfo.getUserId());
            result.put('ownerIsProfile',owner==null?null:owner==UserInfo.getProfileId());
            result.put('nameEqualsOwnerString',name==null || owner==null?null:name.equals(String.valueOf(owner)));
            result.put('nameEqualsOrganizationString',name==null?null:name.equals(String.valueOf(UserInfo.getOrganizationId())));
            result.put('nameEqualsUserString',name==null?null:name.equals(String.valueOf(UserInfo.getUserId())));
            result.put('nameEqualsProfileString',name==null?null:name.equals(String.valueOf(UserInfo.getProfileId())));
        }
        return result;
    }

    private static Map<String,Object> fault(Exception error) {
        return new Map<String,Object>{'exceptionType'=>error.getTypeName(),'exceptionMessage'=>error.getMessage()};
    }
}
