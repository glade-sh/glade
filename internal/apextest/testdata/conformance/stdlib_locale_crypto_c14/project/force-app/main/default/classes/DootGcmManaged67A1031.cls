@IsTest(SeeAllData=false)
private class DootGcmManaged67A1031 {

    private static Map<String,Object> err(String stage, Exception e) {
        return new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()};
    }
    private static void emit(String id, Map<String,Object> o) {
        String j=JSON.serialize(o);
        if(j.length()>600) j=JSON.serialize(new Map<String,Object>{'status'=>'inconclusive_overlength','originalLength'=>j.length()});
        System.assert(false,'CAPTURE|'+id+'|'+j);
    }
    private static String chars(List<Integer> u) { return String.fromCharArray(u); }
    private static Map<String,Object> units(String s) {
        return new Map<String,Object>{'n'=>s.length(),'u'=>s.getChars()};
    }
    @IsTest static void baseline() { System.assertEquals(2,1+1,'BASELINE|arithmetic'); }
    @IsTest static void environment() {
        emit('ENV',new Map<String,Object>{'api'=>'67.0','route'=>'named-test-check-only','selectorNs'=>'gladens001','locale'=>UserInfo.getLocale(),'language'=>UserInfo.getLanguage(),'tz'=>UserInfo.getTimeZone().getID()});
    }
    @IsTest static void observe() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Blob k=Blob.valueOf('0123456789abcdef0123456789abcdef');Blob d=Blob.valueOf('owned');stage='encrypt';Blob c=Crypto.encryptWithManagedIV('AES256-GCM',k,d);o.put('bytes',c.size());stage='decrypt';o.put('roundtrip',Crypto.decryptWithManagedIV('AES256-GCM',k,c).toString()=='owned'); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A21.managed',o);
    }
    @IsTest static void cbcControl() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Blob k=Blob.valueOf('0123456789abcdef0123456789abcdef');Blob iv=Blob.valueOf('0123456789abcdef');Blob d=Blob.valueOf('owned');stage='encrypt';Blob c=Crypto.encrypt('AES256',k,iv,d);stage='decrypt';o.put('roundtrip',Crypto.decrypt('AES256',k,iv,c).toString()=='owned'); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A21.managed.cbc',o);
    }
}
