public class DescribeOwner {
 public static Map<String,Schema.SObjectType> stored;
 public Map<String,Schema.SObjectType> member;
 public static void mutate(Map<String,Schema.SObjectType> m) {m.clear();}
 @AuraEnabled public static String verify() {
{Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> alias = holder;
DescribeOwner.stored = holder;
DescribeOwner instance = new DescribeOwner(); instance.member = holder;
Set<String> saved = holder.keySet();
Integer beforeSize = sibling.size();
String beforeKeys = String.valueOf(sibling.keySet());
holder.clear();
System.assertEquals(0, holder.size(), 'writer');
System.assertEquals(0, alias.size(), 'local alias');
System.assertEquals(0, DescribeOwner.stored.size(), 'static alias');
System.assertEquals(0, instance.member.size(), 'field alias');
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
}{Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> alias = holder;
DescribeOwner.stored = holder;
DescribeOwner instance = new DescribeOwner(); instance.member = holder;
Set<String> saved = holder.keySet();
Integer beforeSize = sibling.size();
String beforeKeys = String.valueOf(sibling.keySet());
alias.clear();
System.assertEquals(0, holder.size(), 'writer');
System.assertEquals(0, alias.size(), 'local alias');
System.assertEquals(0, DescribeOwner.stored.size(), 'static alias');
System.assertEquals(0, instance.member.size(), 'field alias');
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
}{Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> alias = holder;
DescribeOwner.stored = holder;
DescribeOwner instance = new DescribeOwner(); instance.member = holder;
Set<String> saved = holder.keySet();
Integer beforeSize = sibling.size();
String beforeKeys = String.valueOf(sibling.keySet());
DescribeOwner.stored.clear();
System.assertEquals(0, holder.size(), 'writer');
System.assertEquals(0, alias.size(), 'local alias');
System.assertEquals(0, DescribeOwner.stored.size(), 'static alias');
System.assertEquals(0, instance.member.size(), 'field alias');
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
}{Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> alias = holder;
DescribeOwner.stored = holder;
DescribeOwner instance = new DescribeOwner(); instance.member = holder;
Set<String> saved = holder.keySet();
Integer beforeSize = sibling.size();
String beforeKeys = String.valueOf(sibling.keySet());
instance.member.clear();
System.assertEquals(0, holder.size(), 'writer');
System.assertEquals(0, alias.size(), 'local alias');
System.assertEquals(0, DescribeOwner.stored.size(), 'static alias');
System.assertEquals(0, instance.member.size(), 'field alias');
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
}{Map<String,Schema.SObjectType> holder = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> sibling = Schema.getGlobalDescribe();
Map<String,Schema.SObjectType> alias = holder;
DescribeOwner.stored = holder;
DescribeOwner instance = new DescribeOwner(); instance.member = holder;
Set<String> saved = holder.keySet();
Integer beforeSize = sibling.size();
String beforeKeys = String.valueOf(sibling.keySet());
DescribeOwner.mutate(holder);
System.assertEquals(0, holder.size(), 'writer');
System.assertEquals(0, alias.size(), 'local alias');
System.assertEquals(0, DescribeOwner.stored.size(), 'static alias');
System.assertEquals(0, instance.member.size(), 'field alias');
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
}
 Schema.SObjectType token=Schema.getGlobalDescribe().get('account'); token.object='User';
 return 'owned-describe-ok';
 }
 public String getStatus() {return verify();}
}
