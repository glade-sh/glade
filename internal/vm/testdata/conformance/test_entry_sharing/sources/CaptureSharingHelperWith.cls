public with sharing class CaptureSharingHelperWith {
    public static Map<String,Object> observe() {
        Map<String,Object> result=new Map<String,Object>();
        try { result.put('default',[SELECT Id FROM CapturePrivate__c].size()); } catch(Exception e) { result.put('default',e.getTypeName()+': '+e.getMessage()); }
        try { result.put('system',[SELECT Id FROM CapturePrivate__c WITH SYSTEM_MODE].size()); } catch(Exception e) { result.put('system',e.getTypeName()+': '+e.getMessage()); }
        try { result.put('user',[SELECT Id FROM CapturePrivate__c WITH USER_MODE].size()); } catch(Exception e) { result.put('user',e.getTypeName()+': '+e.getMessage()); }
        return result;
    }
}
