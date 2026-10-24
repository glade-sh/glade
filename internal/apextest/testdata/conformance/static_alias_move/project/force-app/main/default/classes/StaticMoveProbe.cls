public class StaticMoveProbe {
    public static List<Account> source;
    public static List<Account> destination;
    public static Map<String,Account> byName;
    public static Account held;
    public static Map<String,Object> observe(String variant) {
        source = new List<Account>();
        destination = new List<Account>();
        byName = new Map<String,Account>();
        held = null;
        Map<String,Object> result = new Map<String,Object>();
        if (variant == 'empty') {
            for (Account row : source) { destination.add(row); row.Description = 'changed'; }
            result.put('sourceSize', source.size());
            result.put('destinationSize', destination.size());
            result.put('mapSize', byName.size());
            result.put('heldNull', held == null);
            return result;
        }
        Account original = new Account(Name = 'Synthetic', Description = 'initial');
        Account other = new Account(Name = 'Other', Description = 'unrelated');
        source.add(original);
        source.add(original);
        source.add(other);
        held = original;
        byName.put('record', original);
        // Warm the static aliases before changing collection membership.
        for (Account row : source) { row.Description = row.Name == 'Synthetic' ? 'warm' : 'unrelated'; }
        Account moved = source.remove(0);
        destination.add(moved);
        destination.add(moved);
        if (variant == 'rejected') {
            try { moved.put('Absent__c', 'rejected'); }
            catch (SObjectException e) { result.put('exception', e.getTypeName()); }
        } else if (variant == 'put') { moved.put('Description', 'changed'); }
        else { moved.Description = 'changed'; }
        result.put('sourceSize', source.size());
        result.put('destinationSize', destination.size());
        result.put('source', source[0].Description);
        result.put('destination', new List<String>{destination[0].Description, destination[1].Description});
        result.put('map', byName.get('record').Description);
        result.put('held', held.Description);
        result.put('original', original.Description);
        result.put('moved', moved.Description);
        result.put('other', source[1].Description);
        return result;
    }
}
