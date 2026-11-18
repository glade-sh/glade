List<String> a = new List<String>(Ord__c.SObjectType.getDescribe().fields.getMap().keySet());
List<String> b = new List<String>(Ord__c.SObjectType.getDescribe().fields.getMap().keySet());
String d = EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(String.join(a, ','))));
List<String> head = new List<String>();
for (Integer i = 0; i < Math.min(8, a.size()); i++) {
    head.add(a[i]);
}
System.debug('OBS|fields|n=' + a.size() + '|first=' + String.join(head, ',') + '|last=' + a[a.size() - 1] + '|d=' + d.left(16) + '|same=' + (String.join(a, ',') == String.join(b, ',')));
