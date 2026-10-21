public class OwnerShapeProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        for (String kind : new List<String>{'Group','User'}) {
            String key = kind == 'Group' ? '00G000000000001' : '005000000000001';
            String given = kind == 'Group' ? 'null' : '"Atlas"';
            Case row = (Case)JSON.deserialize('{"attributes":{"type":"Case"},"OwnerId":"' + key + '","Owner":{"attributes":{"type":"Name"},"Id":"' + key + '","Type":"' + kind + '","FirstName":' + given + '}}', Case.class);
            try { observed.put(kind, row.Owner.FirstName); }
            catch (Exception error) { observed.put(kind, error.getTypeName() + '|' + error.getMessage()); }
        }
        return observed;
    }
}
