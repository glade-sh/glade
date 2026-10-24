public class StandardProfileProbe {
    public static Map<String,String> profiles() {
        Set<String> names = new Set<String>{'Read Only','Contract Manager','Solution Manager','Customer Community User','Customer Community Plus User','Partner Community User'};
        return identities(names);
    }
    public static Map<String,String> boundary() {
        return identities(new Set<String>{'System Administrator','Standard User'});
    }
    private static Map<String,String> identities(Set<String> names) {
        Map<String,String> result = new Map<String,String>();
        for (String name : names) {
            try {
                Profile profile = [SELECT Name,UserType,UserLicense.Name,UserLicense.LicenseDefinitionKey FROM Profile WHERE Name=:name LIMIT 1];
                result.put(profile.Name,profile.UserType+'|'+profile.UserLicense.Name+'|'+profile.UserLicense.LicenseDefinitionKey);
            } catch (QueryException error) {
                result.put(name,error.getTypeName());
            }
        }
        return result;
    }
    public static Map<String,String> missing() {
        Map<String,String> result = new Map<String,String>{'count'=>String.valueOf([SELECT COUNT() FROM Profile WHERE Name='Undeployed Meadow Profile'])};
        try {
            Profile absent = [SELECT Id FROM Profile WHERE Name='Undeployed Meadow Profile' LIMIT 1];
            result.put('singleton','returned');
        } catch (QueryException error) {
            result.put('singleton',error.getTypeName());
            result.put('message',error.getMessage());
        }
        return result;
    }
}
