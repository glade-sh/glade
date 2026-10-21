public class SetupQueryPrerequisiteProbe {
    public static Map<String,Object> metadata() {
        Map<String,Object> row = new Map<String,Object>();
        row.put('presentGroup',[SELECT COUNT() FROM PermissionSetGroup WHERE DeveloperName='Meadow_Query_Group']);
        row.put('presentPermissionSet',[SELECT COUNT() FROM PermissionSet WHERE Name='Meadow_Query_Access']);
        row.put('absentGroup',[SELECT COUNT() FROM PermissionSetGroup WHERE DeveloperName='Meadow_Undeployed_Access']);
        try {
            PermissionSetGroup missing = [SELECT Id FROM PermissionSetGroup WHERE DeveloperName='Meadow_Undeployed_Access' LIMIT 1];
            row.put('missingGroupSingleton',true);
        } catch (Exception error) {
            row.put('missingGroupException',error.getTypeName());
            row.put('missingGroupMessage',error.getMessage());
        }
        return row;
    }
    public static Map<String,Object> community() {
        List<UserLicense> licenses=[SELECT Id,Name,LicenseDefinitionKey,Status,TotalLicenses FROM UserLicense WHERE LicenseDefinitionKey='PID_Customer_Community'];
        Map<String,Object> row=new Map<String,Object>{'count'=>licenses.size()};
        if(!licenses.isEmpty()) {
            UserLicense license=licenses[0];
            row.put('name',license.Name);row.put('key',license.LicenseDefinitionKey);
            row.put('status',license.Status);row.put('total',license.TotalLicenses);
            List<Profile> profiles=[SELECT Name,UserType FROM Profile WHERE UserLicenseId=:license.Id ORDER BY Name];
            row.put('profileCount',profiles.size());
            if(!profiles.isEmpty()) {row.put('profileName',profiles[0].Name);row.put('userType',profiles[0].UserType);}
        }
        return row;
    }
    public static Map<String,Object> packageLicenses() {
        Map<String,Object> row=new Map<String,Object>();
        List<PermissionSetLicense> licenses=[SELECT Id,DeveloperName,MasterLabel,Status,TotalLicenses FROM PermissionSetLicense WHERE DeveloperName IN ('SalesforceCPQ_CPQStandardPerm','SalesforceCPQ_CPQAAPerm')];
        row.put('count',licenses.size());
        for(PermissionSetLicense license:licenses) {
            row.put(license.DeveloperName,new Map<String,Object>{'label'=>license.MasterLabel,'status'=>license.Status,'total'=>license.TotalLicenses,'prefix'=>String.valueOf(license.Id).left(3)});
        }
        return row;
    }
}
