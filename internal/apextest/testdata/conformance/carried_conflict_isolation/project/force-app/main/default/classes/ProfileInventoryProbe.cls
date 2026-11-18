public class ProfileInventoryProbe {
    public static Object observe(String shape) {
        if (shape == 'readOnly') return [SELECT COUNT() FROM Profile WHERE Name = 'Read Only'];
        if (shape == 'missing') {
            try { Profile p = [SELECT Name FROM Profile WHERE Name = 'Absent Native Profile']; return p.Name; }
            catch (Exception e) { return new Map<String,Object>{'exceptionType'=>e.getTypeName(),'exceptionMessage'=>e.getMessage()}; }
        }
        List<Object> out = new List<Object>();
        List<Profile> records;
        if (shape == 'missing') records = [SELECT Name, UserLicense.Name FROM Profile WHERE Name = 'Absent Native Profile' ORDER BY Name];
        else if (shape == 'limited') records = [SELECT Name, UserLicense.Name FROM Profile ORDER BY Name LIMIT 1];
        else records = [SELECT Name, UserLicense.Name FROM Profile ORDER BY Name];
        for (Profile p : records) out.add(new Map<String,Object>{'name'=>p.Name, 'license'=>p.UserLicense.Name});
        return out;
    }
}
