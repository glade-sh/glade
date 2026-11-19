List<String> a = new List<String>(Schema.getGlobalDescribe().keySet());
List<String> b = new List<String>(Schema.getGlobalDescribe().keySet());
String d = EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(String.join(a, ','))));
List<String> head = new List<String>();
for (Integer i = 0; i < Math.min(6, a.size()); i++) {
    head.add(a[i]);
}
List<String> tail = new List<String>();
for (Integer i = Math.max(0, a.size() - 6); i < a.size(); i++) {
    tail.add(a[i]);
}
System.debug('OBS|global|n=' + a.size() + '|first=' + String.join(head, ',') + '|last=' + String.join(tail, ',') + '|d=' + d.left(16) + '|same=' + (String.join(a, ',') == String.join(b, ',')));
