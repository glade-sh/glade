public class ProfileProbe {
    public static Object observe() {
        List<Object> selected = new List<Object>();
        for (Profile p : [SELECT Name, UserLicense.Name FROM Profile ORDER BY Name LIMIT 1]) {
            selected.add(new Map<String, Object>{'name'=>p.Name, 'license'=>p.UserLicense.Name});
        }
        return selected;
    }
}
