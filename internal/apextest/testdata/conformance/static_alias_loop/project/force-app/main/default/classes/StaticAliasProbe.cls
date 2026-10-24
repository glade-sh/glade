public class StaticAliasProbe {
    public static List<Account> rows;
    public static Map<String,Object> observe(String variant) {
        rows = new List<Account>();
        Map<String,Object> result = new Map<String,Object>();
        if (variant == 'empty') {
            for (Account row : rows) { row.Description = 'processed'; }
            result.put('size', rows.size());
            return result;
        }
        for (Integer i = 0; i < 3; i++) {
            Account row = (Account)Account.SObjectType.newSObject();
            row.Name = 'Synthetic ' + i;
            rows.add(row);
            rows.add(row);
        }
        Account first = rows[0];
        if (variant == 'nested') { rows[2].Parent = first; }
        if (variant == 'move') { rows.add(rows.remove(0)); }
        if (variant == 'rejected') {
            try { first.put('Absent__c', 'rejected'); }
            catch (SObjectException e) { result.put('exception', e.getTypeName()); }
            result.put('size', rows.size());
            result.put('unchanged', rows[1].Name == 'Synthetic 0');
            return result;
        }
        Integer sequence = 0;
        for (Account row : rows) {
            if (variant == 'put') { row.put('Description', 'step-' + sequence); }
            else { row.Description = 'step-' + sequence; }
            sequence++;
        }
        List<String> descriptions = new List<String>();
        for (Account row : rows) { descriptions.add(row.Description); }
        result.put('descriptions', descriptions);
        result.put('first', first.Description);
        result.put('size', rows.size());
        if (variant == 'nested') { result.put('parent', rows[2].Parent.Description); }
        return result;
    }
}
