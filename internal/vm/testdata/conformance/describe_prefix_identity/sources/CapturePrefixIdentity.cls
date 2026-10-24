public class CapturePrefixIdentity {
    public static Map<String,Object> observe() {
        String x=CaptureIdentity__c.SObjectType.getDescribe().getKeyPrefix();
        String y=CaptureSecond__c.SObjectType.getDescribe().getKeyPrefix();
        String u=User.SObjectType.getDescribe().getKeyPrefix();
        String p=Profile.SObjectType.getDescribe().getKeyPrefix();
        String a=Account.SObjectType.getDescribe().getKeyPrefix();
        Id fake=(Id)(x+'000000000001');
        Map<String,Object> rows=new Map<String,Object>{'unique'=>new Set<String>{x,y,u,p,a}.size()==5,
            'userPrefix'=>u,'profilePrefix'=>p,'accountPrefix'=>a,'customPrefixWidth'=>x.length(),
            'customType'=>String.valueOf(fake.getSObjectType()),'userCollision'=>((Id)(u+'000000000001'))==UserInfo.getUserId(),
            'profileCollision'=>((Id)(p+'000000000001'))==UserInfo.getProfileId(),
            'customCounterCount'=>[SELECT COUNT() FROM CaptureIdentity__c WHERE Id=:fake]};
        try { Id bad=(Id)'bad'; rows.put('invalid','accepted'); } catch(Exception e) { rows.put('invalid',e.getTypeName()+': '+e.getMessage()); }
        return rows;
    }
}
