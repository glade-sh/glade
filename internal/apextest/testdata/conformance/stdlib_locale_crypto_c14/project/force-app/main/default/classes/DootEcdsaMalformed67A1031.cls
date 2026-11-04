@IsTest(SeeAllData=false)
private class DootEcdsaMalformed67A1031 {

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
    @IsTest static void badSign() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Blob d=Blob.valueOf('owned');stage='sign';Blob s=Crypto.sign('ECDSA-SHA256',d,Blob.valueOf('not a key'));o.put('bytes',s.size());o.put('digest',EncodingUtil.convertToHex(s)==EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',d))); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A23.sign',o);
    }
    @IsTest static void badVerify() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Blob d=Blob.valueOf('owned');Blob raw=Crypto.generateDigest('SHA-256',d);stage='verify';o.put('out',Crypto.verify('ECDSA-SHA256',d,raw,Blob.valueOf('not a public key'))); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A23.verify',o);
    }
    @IsTest static void encodingControl() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Blob d=Blob.valueOf('owned');stage='encoding';o.put('hex',EncodingUtil.convertToHex(d));o.put('hexRoundtrip',EncodingUtil.convertFromHex(EncodingUtil.convertToHex(d)).toString()=='owned');o.put('base64Roundtrip',EncodingUtil.base64Decode(EncodingUtil.base64Encode(d)).toString()=='owned');o.put('digestBytes',Crypto.generateDigest('SHA-256',d).size()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A23.encodingControl',o);
    }
}
