public class MdtRelProbe {
    static String text(Object o) { return o == null ? 'null' : String.valueOf(o); }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        MdtRelProbe__mdt direct = new MdtRelProbe__mdt();
        try { direct.Target__c = 'Account'; m.put('directAssign', text(direct.Target__c)); } catch (Exception e) { m.put('directAssign', e.getTypeName() + ':' + e.getMessage()); }
        MdtRelProbe__mdt viaPut = new MdtRelProbe__mdt();
        try { viaPut.put(MdtRelProbe__mdt.Target__c, 'Contact'); m.put('putToken', text(viaPut.Target__c)); } catch (Exception e) { m.put('putToken', e.getTypeName() + ':' + e.getMessage()); }
        MdtRelProbe__mdt viaName = new MdtRelProbe__mdt();
        try { viaName.put('Target__c', 'Lead'); m.put('putName', text(viaName.get('Target__c'))); } catch (Exception e) { m.put('putName', e.getTypeName() + ':' + e.getMessage()); }
        try {
            MdtRelProbe__mdt viaJson = (MdtRelProbe__mdt) JSON.deserialize('{"Target__c":"Opportunity","MasterLabel":"J","DeveloperName":"J"}', MdtRelProbe__mdt.class);
            m.put('jsonTarget', text(viaJson.Target__c));
            m.put('jsonLabel', text(viaJson.MasterLabel));
            m.put('jsonRelationship', viaJson.Target__r == null ? 'null' : 'set');
            m.put('jsonSerialized', JSON.serialize(viaJson).contains('Opportunity'));
        } catch (Exception e) { m.put('jsonTarget', e.getTypeName() + ':' + e.getMessage()); }
        try {
            Map<String,Object> raw = new Map<String,Object>{ 'Target__c' => 'Case', 'MasterLabel' => 'K' };
            MdtRelProbe__mdt viaMap = (MdtRelProbe__mdt) JSON.deserialize(JSON.serialize(raw), MdtRelProbe__mdt.class);
            m.put('mapJsonTarget', text(viaMap.Target__c));
        } catch (Exception e) { m.put('mapJsonTarget', e.getTypeName() + ':' + e.getMessage()); }
        Schema.DescribeFieldResult f = MdtRelProbe__mdt.Target__c.getDescribe();
        m.put('fieldType', String.valueOf(f.getType()));
        m.put('referenceTo', String.valueOf(f.getReferenceTo()));
        m.put('relationshipName', text(f.getRelationshipName()));
        return m;
    }
}
