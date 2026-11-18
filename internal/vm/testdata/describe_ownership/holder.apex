Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> alias = holder;
DescribeOwner.stored = holder;
DescribeOwner instance = new DescribeOwner(); instance.member = holder;
Set<String> saved = holder.keySet();
Integer beforeSize = sibling.size();
String beforeKeys = String.valueOf(sibling.keySet());
@@WRITE@@
System.assertEquals(@@SIZE@@, holder.size(), 'writer');
System.assertEquals(@@SIZE@@, alias.size(), 'local alias');
System.assertEquals(@@SIZE@@, DescribeOwner.stored.size(), 'static alias');
System.assertEquals(@@SIZE@@, instance.member.size(), 'field alias');
// A saved global describe key set writes a removal through to its holder and
// the holder's aliases, never to a sibling or a fresh getGlobalDescribe()
// (native review_round10 g_gd_remove, APIs 62.0 and 67.0: N to N-1).
Integer writerSize = holder.size();
Integer savedSize = holder.containsKey('account') ? writerSize - 1 : writerSize;
saved.remove('account');
System.assertEquals(savedSize, holder.size(), 'saved global key set');
System.assertEquals(false, holder.containsKey('account'), 'saved global key set removes the key');
System.assertEquals(savedSize, alias.size());
System.assertEquals(savedSize, DescribeOwner.stored.size());
System.assertEquals(savedSize, instance.member.size());
System.assertEquals(beforeSize, sibling.size(), 'independent holder size');
System.assertEquals(beforeKeys, String.valueOf(sibling.keySet()), 'independent holder keys');
System.assertEquals(Account.SObjectType, sibling.get('account'), 'independent holder token');
System.assertEquals(beforeSize, Schema.getGlobalDescribe().size(), 'raw cache');
