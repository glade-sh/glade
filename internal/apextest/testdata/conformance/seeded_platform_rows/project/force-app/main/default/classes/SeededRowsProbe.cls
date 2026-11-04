public class SeededRowsProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        for (String objectName : new List<String>{'DatedConversionRate','PlatformCachePartition'}) {
            try {
                result.put(objectName + '.count', Database.countQuery('SELECT count() FROM ' + objectName));
                result.put(objectName + '.limitZero', Database.query('SELECT Id FROM ' + objectName + ' LIMIT 0').size());
                try {
                    SObject row = Database.query('SELECT Id FROM ' + objectName + ' LIMIT 1');
                    result.put(objectName + '.singleton', 'row');
                } catch (Exception e) {
                    result.put(objectName + '.singleton', e.getTypeName() + '|' + e.getMessage());
                }
            } catch (Exception e) {
                result.put(objectName + '.countError', e.getTypeName());
            }
        }
        return result;
    }
}
