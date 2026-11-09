public class ClassDatesProbe {
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        DateTime n = System.now();
        ApexClass c = [SELECT Name, CreatedDate, LastModifiedDate, SystemModstamp, CreatedById, LastModifiedById FROM ApexClass WHERE Name = 'ClassDatesProbe'];
        o.put('lmdNull', c.LastModifiedDate == null);
        o.put('createdNull', c.CreatedDate == null);
        o.put('modstampNull', c.SystemModstamp == null);
        o.put('lmdNotAfterNow', c.LastModifiedDate != null && c.LastModifiedDate <= n);
        o.put('createdNotAfterNow', c.CreatedDate != null && c.CreatedDate <= n);
        o.put('lmdNotBeforeCreated', c.LastModifiedDate != null && c.CreatedDate != null && c.LastModifiedDate >= c.CreatedDate);
        o.put('lmdByIdNull', c.LastModifiedById == null);
        o.put('createdByIdNull', c.CreatedById == null);
        List<ApexClass> filtered = [SELECT Id FROM ApexClass WHERE Name = 'ClassDatesProbe' AND LastModifiedDate <= :n];
        o.put('filterLmdLeNowCount', filtered.size());
        List<ApexClass> ordered = [SELECT Name FROM ApexClass WHERE Name IN ('ClassDatesProbe', 'ClassDatesProbeTest') ORDER BY LastModifiedDate DESC, Name ASC];
        o.put('orderedCount', ordered.size());
        return o;
    }
}
