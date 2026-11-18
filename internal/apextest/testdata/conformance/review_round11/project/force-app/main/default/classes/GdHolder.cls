public class GdHolder {
 public static Map<String,Schema.SObjectType> stored;
 public Map<String,Schema.SObjectType> member;
 public static void mutate(Map<String,Schema.SObjectType> m) { m.keySet().remove('opportunity'); }
 public static String gk_get() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Map<String,Schema.SObjectType> other = Schema.getGlobalDescribe(); Set<String> k = gd.keySet(); Integer n = gd.size();
k.remove('account');
String o = '' + gd.get('account') + '|' + gd.get('Account') + '|' + gd.containsKey('Account') + '|' + (n - gd.size()) + '|' + other.get('account') + '|' + other.containsKey('account') + '|' + (n - other.size()) + '|' + k.contains('Account') + '|' + Schema.getGlobalDescribe().get('account');
 return o;
 }
 public static String gk_direct() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size(); Set<String> pre = gd.keySet();
gd.remove('account');
String o = '' + gd.get('account') + '|' + gd.containsKey('account') + '|' + (n - gd.size()) + '|' + gd.keySet().contains('account') + '|' + pre.contains('account') + '|' + gd.get('Account') + '|' + Schema.getGlobalDescribe().containsKey('account');
 return o;
 }
 public static String gk_p2a() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
gd.put('zzshare', Account.SObjectType); gd.remove('account');
String o = '' + gd.containsKey('zzshare') + '|' + (gd.size() - n) + '|' + gd.get('account') + '|' + Schema.getGlobalDescribe().containsKey('zzshare') + '|' + (Schema.getGlobalDescribe().size() - n);
 return o;
 }
 public static String gk_direct_upper() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
Object r = gd.remove('Account');
String o = '' + r + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + (n - gd.size()) + '|' + gd.keySet().contains('account');
 return o;
 }
 public static String gk_removeall() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
Boolean r = gd.keySet().removeAll(new Set<String>{'account'});
String o = '' + r + '|' + (n - gd.size()) + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + Schema.getGlobalDescribe().containsKey('account') + '|' + (Schema.getGlobalDescribe().size() - n);
 return o;
 }
 public static String gk_retainall() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size();
Boolean r = gd.keySet().retainAll(new Set<String>{'user'});
String o = '' + r + '|' + gd.size() + '|' + gd.containsKey('user') + '|' + gd.get('user') + '|' + gd.containsKey('account') + '|' + gd.get('account') + '|' + gd.keySet() + '|' + (Schema.getGlobalDescribe().size() - n);
 return o;
 }
 public static String gk_saved() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Set<String> saved = gd.keySet(); Set<String> saved2 = gd.keySet(); Integer n = gd.size();
saved.remove('account');
String o = '' + (n - gd.size()) + '|' + gd.containsKey('account') + '|' + saved2.contains('account') + '|' + (n - saved2.size()) + '|' + (n - saved.size());
gd.clear(); Boolean r = saved.remove('user');
o += '|' + r + '|' + gd.size() + '|' + saved.size() + '|' + saved2.size() + '|' + (Schema.getGlobalDescribe().size() - n);
 return o;
 }
 public static String gk_alias() {
Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Map<String,Schema.SObjectType> alias = gd; GdHolder.stored = gd; GdHolder h = new GdHolder(); h.member = gd; Integer n = gd.size();
gd.keySet().remove('account');
String o = '' + (n - alias.size()) + '|' + alias.containsKey('account') + '|' + (n - GdHolder.stored.size()) + '|' + GdHolder.stored.containsKey('account') + '|' + (n - h.member.size()) + '|' + h.member.containsKey('account');
alias.keySet().remove('user');
o += '|' + gd.containsKey('user') + '|' + (n - gd.size());
GdHolder.stored.keySet().remove('contact');
o += '|' + gd.containsKey('contact') + '|' + h.member.containsKey('contact');
h.member.keySet().removeAll(new Set<String>{'lead'});
o += '|' + gd.containsKey('lead') + '|' + alias.containsKey('lead');
GdHolder.mutate(gd);
o += '|' + gd.containsKey('opportunity') + '|' + (n - gd.size()) + '|' + Schema.getGlobalDescribe().containsKey('account') + '|' + (Schema.getGlobalDescribe().size() - n);
 return o;
 }
 public static String run() {
  List<String> out = new List<String>();
  out.add('gk_get=' + gk_get());
  out.add('gk_removeall=' + gk_removeall());
  out.add('gk_retainall=' + gk_retainall());
  out.add('gk_saved=' + gk_saved());
  out.add('gk_alias=' + gk_alias());
  return String.join(out, '~');
 }
}