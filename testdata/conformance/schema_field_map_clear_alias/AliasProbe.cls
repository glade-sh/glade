public class AliasProbe {
    static Map<String,Schema.SObjectField> stored;
    Map<String,Schema.SObjectField> member;
    static void clearArg(Map<String,Schema.SObjectField> a) { a.clear(); System.debug('ALIAS|A04|' + a.isEmpty()); System.assertEquals(true, a.isEmpty()); }
    public static void run() {
        Map<String,Schema.SObjectField> d=Schema.SObjectType.Account.fields.getMap();
        Map<String,Schema.SObjectField> localAlias=d;
        Map<String,Schema.SObjectField> other=Schema.SObjectType.Account.fields.getMap();
        Map<String,Schema.SObjectField> cloned=d.clone();
        AliasProbe holder=new AliasProbe(); holder.member=d; stored=d;
        clearArg(d);
        System.debug('ALIAS|A01|' + (localAlias.isEmpty() && d.isEmpty())); System.assertEquals(true, localAlias.isEmpty() && d.isEmpty(), 'A01');
        System.debug('ALIAS|A02|' + stored.isEmpty()); System.assertEquals(true, stored.isEmpty(), 'A02');
        System.debug('ALIAS|A03|' + holder.member.isEmpty()); System.assertEquals(true, holder.member.isEmpty(), 'A03');
        System.debug('ALIAS|A05|' + other.containsKey('Name')); System.assertEquals(true, other.containsKey('Name'), 'A05');
        System.debug('ALIAS|A06|' + Schema.SObjectType.Account.fields.getMap().containsKey('Name')); System.assertEquals(true, Schema.SObjectType.Account.fields.getMap().containsKey('Name'), 'A06');
        System.debug('ALIAS|A07|' + cloned.containsKey('Name')); System.assertEquals(true, cloned.containsKey('Name'), 'A07');
        Map<String,Integer> m=new Map<String,Integer>{'a'=>1}; Map<String,Integer> a=m; m.clear();
        System.debug('ALIAS|A08|' + a.isEmpty()); System.assertEquals(true, a.isEmpty(), 'A08');
    }
}
