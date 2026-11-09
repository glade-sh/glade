public without sharing class SynthFlsProbe67 {
    private static final String CLASS_API = '67.0';
    private static final Set<String> SCOPE = new Set<String>{'SynthFlsParent__c','SynthFlsChild__c','Account','Contact'};
    private static final Set<String> CUSTOM = new Set<String>{'SynthFlsParent__c','SynthFlsChild__c'};
    private static final Set<String> PROFILES = new Set<String>{'System Administrator','Standard User'};
    private static String enc(String value) { return EncodingUtil.base64Encode(Blob.valueOf(value)); }
    private static String profileName(SObject p) {
        if (p == null) return null;
        SObject profile = p.getSObject('Profile');
        return profile == null ? null : (String)profile.get('Name');
    }
    public static String capture(String phase, String route, String dataset) {
        List<String> payload = new List<String>();
        if (dataset == 'context') {
            payload.add(JSON.serialize(new Map<String,Object>{'kind'=>'context','organizationId'=>UserInfo.getOrganizationId(), 'userId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId(),'locale'=>UserInfo.getLocale(),'timezone'=>UserInfo.getTimeZone().getID(),'language'=>UserInfo.getLanguage(),'isTest'=>Test.isRunningTest(),'classApi'=>CLASS_API}));
            for (Profile p : [SELECT Id,Name FROM Profile WHERE Name IN :PROFILES ORDER BY Name,Id]) payload.add(JSON.serialize(new Map<String,Object>{'kind'=>'targetProfile','row'=>p}));
            for (PermissionSet p : [SELECT Id,Name,IsOwnedByProfile,ProfileId,Profile.Name FROM PermissionSet WHERE Profile.Name IN :PROFILES OR Name = 'SynthFlsUnassigned' ORDER BY Id]) payload.add(JSON.serialize(new Map<String,Object>{'kind'=>'targetParent','row'=>p}));
            for (PermissionSetAssignment a : [SELECT Id,AssigneeId,PermissionSetId,PermissionSet.Name FROM PermissionSetAssignment WHERE PermissionSet.Name = 'SynthFlsUnassigned' ORDER BY Id]) payload.add(JSON.serialize(new Map<String,Object>{'kind'=>'assignment','row'=>a}));
        } else if (dataset == 'fields' || dataset == 'objects') {
            String q = dataset == 'fields'
                ? 'SELECT Id,ParentId,SobjectType,Field,PermissionsRead,PermissionsEdit,Parent.Name,Parent.IsOwnedByProfile,Parent.ProfileId,Parent.Profile.Name FROM FieldPermissions WHERE SobjectType IN :SCOPE ORDER BY ParentId,SobjectType,Field,Id'
                : 'SELECT Id,ParentId,SobjectType,PermissionsRead,PermissionsCreate,PermissionsEdit,PermissionsDelete,PermissionsViewAllRecords,PermissionsModifyAllRecords,Parent.Name,Parent.IsOwnedByProfile,Parent.ProfileId,Parent.Profile.Name FROM ObjectPermissions WHERE SobjectType IN :SCOPE ORDER BY ParentId,SobjectType,Id';
            List<SObject> rows = Database.query(q);
            Map<String,SObject> parents = new Map<String,SObject>();
            Map<String,Integer> counts = new Map<String,Integer>();
            Map<String,Map<String,Integer>> countsByObject = new Map<String,Map<String,Integer>>();
            // Include explicitly targeted profile/permission-set parents even when no rows exist.
            for (PermissionSet p : [SELECT Id,Name,IsOwnedByProfile,ProfileId,Profile.Name FROM PermissionSet WHERE Profile.Name IN :PROFILES OR Name = 'SynthFlsUnassigned' ORDER BY Id]) {
                String key=(String)p.Id; parents.put(key,p);counts.put(key,0);countsByObject.put(key,new Map<String,Integer>());
            }
            List<String> selected = new List<String>();
            for (SObject row : rows) {
                String key=(String)row.get('ParentId'); SObject p=row.getSObject('Parent');
                if (!parents.containsKey(key)) {parents.put(key,p);counts.put(key,0);countsByObject.put(key,new Map<String,Integer>());}
                counts.put(key,counts.get(key)+1);
                String objectName=(String)row.get('SobjectType');
                Map<String,Integer> byObject=countsByObject.get(key);
                byObject.put(objectName,(byObject.containsKey(objectName)?byObject.get(objectName):0)+1);
                if (CUSTOM.contains(objectName) || PROFILES.contains(profileName(p))) selected.add(JSON.serialize(new Map<String,Object>{'kind'=>'permissionRow','row'=>row}));
            }
            payload.add(JSON.serialize(new Map<String,Object>{'kind'=>'query','query'=>q,'scope'=>new List<String>{'SynthFlsParent__c','SynthFlsChild__c','Account','Contact'},'totalRows'=>rows.size(),'selectedRows'=>selected.size()}));
            List<String> keys=new List<String>(parents.keySet());keys.sort();
            for (String key : keys) payload.add(JSON.serialize(new Map<String,Object>{'kind'=>'parentCount','parentId'=>key,'parent'=>parents.get(key),'count'=>counts.get(key),'byObject'=>countsByObject.get(key)}));
            payload.addAll(selected);
        } else { throw new IllegalArgumentException('Unknown dataset: '+dataset); }
        // Each row is a bounded independent chunk. Reject oversized captures before
        // emitting partial evidence; do not silently truncate or assume log capacity.
        Integer encodedBytes=0;
        for(String row : payload) {
            Integer n=enc(row).length();
            if(n>12000) throw new IllegalArgumentException('SYNTH_FLS_CARRIER_CHUNK_LIMIT');
            encodedBytes += n+100;
        }
        if(encodedBytes>1000000) throw new IllegalArgumentException('SYNTH_FLS_CARRIER_TRANSACTION_LIMIT');
        for(Integer i=0;i<payload.size();i++) System.debug(LoggingLevel.ERROR,'SYNTH_FLS_ROW|'+dataset+'|'+String.valueOf(i)+'|'+enc(payload[i]));
        String digest=EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',Blob.valueOf(String.join(payload,'\n'))));
        String endManifest='SYNTH_FLS_END|'+enc(JSON.serialize(new Map<String,Object>{'phase'=>phase,'route'=>route,'dataset'=>dataset,'classApi'=>CLASS_API,'organizationId'=>UserInfo.getOrganizationId(),'userId'=>UserInfo.getUserId(),'profileId'=>UserInfo.getProfileId(),'payloadRows'=>payload.size(),'sha256'=>digest}));
        System.debug(LoggingLevel.ERROR,endManifest);
        return endManifest;
    }
}
