public class SchemaTokenHashProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        SObjectType a = Account.SObjectType;
        SObjectType b = ((SObject) Type.forName('Account').newInstance()).getSObjectType();
        SObjectType c = Schema.getGlobalDescribe().get('account');
        SObjectType d = Contact.SObjectType;
        SObjectField f1 = Account.Name;
        SObjectField f2 = Schema.SObjectType.Account.fields.getMap().get('name');
        SObjectField f3 = Account.AccountNumber;
        o.put('typeEquals', a.equals(b));
        o.put('typeHashSame', System.hashCode(a) == System.hashCode(b));
        o.put('typeHashSameGlobal', System.hashCode(a) == System.hashCode(c));
        o.put('typeHashSameDescribe', System.hashCode(a) == System.hashCode(Account.SObjectType.getDescribe().getSObjectType()));
        o.put('typeHashDiffers', System.hashCode(a) != System.hashCode(d));
        o.put('fieldEquals', f1.equals(f2));
        o.put('fieldHashSame', System.hashCode(f1) == System.hashCode(f2));
        o.put('fieldHashDiffers', System.hashCode(f1) != System.hashCode(f3));
        o.put('classLiteralHashSame', System.hashCode(Type.forName('Account')) == System.hashCode(Account.class));
        o.put('setContainsType', new Set<SObjectType>{ a }.contains(b));
        o.put('mapContainsType', new Map<SObjectType, Integer>{ a => 1 }.containsKey(b));
        o.put('setContainsField', new Set<SObjectField>{ f1 }.contains(f2));
        return o;
    }
}
