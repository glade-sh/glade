public class JwtAlgorithmProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> out = new Map<String,Object>();
  String header = EncodingUtil.base64Encode(Blob.valueOf('{"alg":"HS256","typ":"JWT"}')).replace('=','').replace('+','-').replace('/','_');
  String payload = EncodingUtil.base64Encode(Blob.valueOf('{"aud":"urn:owned"}')).replace('=','').replace('+','-').replace('/','_');
  String input=header+'.'+payload;
  List<String> keys=new List<String>{'invalid!',EncodingUtil.base64Encode(Blob.valueOf('owned-key')),'MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjdaJs2FKa93m+WXpvArIO/4yHLPyoUyENa3k8q8N6GJgTrVg7eZ2VWB739yS90Ipw997NBXIVCkVr4edIkjdmvRswsjRCsNpx6/L6wM+pXGX9sG0jjMgZQbDzZhOH7SSXSv4ywL0jX8NiyKkheHBPj/z633WJNaQ9FiFbtg3sSuvk9ERBLH0jK8NOWp+NK1lOGMaOi6oxXmtCr4tFsSuzv3KvX7I+zI5Byu2KlljXQcGK+DW5lVejq7XhQYQpPQoUfiSsAo6djYRuOEc6k20YDBaUeUAJeXp0GrGuyOCFUfZvriS3/lPDdf51fkJXqI6Py4/FRxDaHgbWY0fd6rBlQIDAQAB'};
  for(Integer i=0;i<keys.size();i++) {
   try { Auth.JWT result=Auth.JWTUtil.validateJWTWithKey(input+'.c2ln',keys[i]); out.put('key'+i,'returned'); }
   catch(Exception e) { out.put('key'+i,e.getTypeName()+':'+e.getMessage()); }
  }
  Blob mac=Crypto.generateMac('HmacSHA256',Blob.valueOf(input),Blob.valueOf('owned-key'));
  String sig=EncodingUtil.base64Encode(mac).replace('=','').replace('+','-').replace('/','_');
  try { Auth.JWT result=Auth.JWTUtil.validateJWTWithKey(input+'.'+sig,keys[1]); out.put('validHmac','returned'); }
  catch(Exception e) { out.put('validHmac',e.getTypeName()+':'+e.getMessage()); }
  String rsaPublicKey='MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtl9aMAPSFN+/czzP1iCWQemnvyu1KuzH5gSlCuIGhzFXPYBn093WywTZ9Sd/Mrc8EIPvMLzSGkVH6XLVTrpj2jJnJjE9NRZyADwlZt+pHjgjM8i3jbUGJ+6tEqPrs28fgvrwB6hSTXN38YoIGt/deXYvjaJXmK8i0rKGrPRDm56PTHqHeyibdqcHxKSNm/Forb/NogMmCnwtPmgiwmf1jgtSgCOki0kfpW2hfT8biZX9CRLPJJtPdVz6uxw8XT0dKRrfUgTjroZB9OhI9aMfxDtLFeQI5nKV7f0Tt6qdED8L3MJFOzb6hpfBclLjhSYGX++CKrx8ZHvbQ7tjjRiwxwIDAQAB';
  try { Auth.JWT result=Auth.JWTUtil.validateJWTWithKey('eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJ1cm46b3duZWQifQ.IaF_yFQB8r6hX0NYEvabSc9TjwiibJjBrinkJt2JodTQOptUsYuCTNSsPcp-SiEhE0Uw3REb5zyvLAOpLb-VUrgtGjYIlBRwlHNSPRFhSYYG7RV-iXxG04ptx3U8Uai2n_kGJsvbjSkNKyKZ5KNWvO83Iui2MwE2uSqKdUjdg8K5I6tVsx5wpyn8MBqiHjTbx19hAX6vvjsAlRM74RnCsI0rXGlOKeBUe6z-dlei8U427WIan7feWWnjR9EcD8xBUPOPO_sdj7sajpCj6onGY7zYdH8gWBLOdoK8C9eOB7GA4cT7HRGRsxMGGzoJtI3w2eWndu5IrRJXvo0eDfjYkw',rsaPublicKey); out.put('rsaSignedHeader','returned'); }
  catch(Exception e) { out.put('rsaSignedHeader',e.getTypeName()+':'+e.getMessage()); }
  try { Auth.JWT result=Auth.JWTUtil.validateJWTWithKey('eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJ1cm46b3duZWQifQ.mvqS10VBQyWj3gNHn4DK63M2YL93sdDB3ehcNIqJ--I',rsaPublicKey); out.put('rsaKeyHmac','returned'); }
  catch(Exception e) { out.put('rsaKeyHmac',e.getTypeName()+':'+e.getMessage()); }
  return out;
 }
}
