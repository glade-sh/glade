List<String> a = new List<String>(Account.SObjectType.getDescribe().fields.getMap().keySet());
String d = EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(String.join(a, ','))));
List<String> head = new List<String>();
for (Integer i = 0; i < Math.min(6, a.size()); i++) {
    head.add(a[i]);
}
System.debug('OBS|accfields|n=' + a.size() + '|first=' + String.join(head, ',') + '|last=' + a[a.size() - 1] + '|d=' + d.left(16));
