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
// Global-describe key sets are release-baseline snapshots. App/module live
// views have separate controls. A saved snapshot removal must stay local.
Integer writerSize = holder.size();
saved.remove('account');
System.assertEquals(writerSize, holder.size(), 'saved global snapshot');
System.assertEquals(writerSize, alias.size());
System.assertEquals(writerSize, DescribeOwner.stored.size());
System.assertEquals(writerSize, instance.member.size());
System.assertEquals(beforeSize, sibling.size(), 'independent holder size');
System.assertEquals(beforeKeys, String.valueOf(sibling.keySet()), 'independent holder keys');
System.assertEquals(Account.SObjectType, sibling.get('account'), 'independent holder token');
System.assertEquals(beforeSize, Schema.getGlobalDescribe().size(), 'raw cache');
