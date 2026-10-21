public class LogAccessProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> observed = new Map<String,Object>();
        Schema.DescribeSObjectResult logInfo = ApexLog.SObjectType.getDescribe();
        Schema.DescribeSObjectResult contactInfo = Contact.SObjectType.getDescribe();
        Schema.DescribeFieldResult logStatus = ApexLog.Status.getDescribe();
        Schema.DescribeFieldResult contactName = Contact.LastName.getDescribe();
        observed.put('apexLogCrud', new List<Boolean>{logInfo.isAccessible(), logInfo.isCreateable(), logInfo.isUpdateable(), logInfo.isDeletable()});
        observed.put('contactCrudControl', new List<Boolean>{contactInfo.isAccessible(), contactInfo.isCreateable(), contactInfo.isUpdateable(), contactInfo.isDeletable()});
        observed.put('apexLogStatusAccess', new List<Boolean>{logStatus.isAccessible(), logStatus.isCreateable(), logStatus.isUpdateable()});
        observed.put('contactLastNameAccessControl', new List<Boolean>{contactName.isAccessible(), contactName.isCreateable(), contactName.isUpdateable()});
        observed.put('createGuard', classify(logInfo.isCreateable(), logStatus.isCreateable()));
        observed.put('updateGuard', classify(logInfo.isUpdateable(), logStatus.isUpdateable()));
        observed.put('contactCreateGuardControl', classify(contactInfo.isCreateable(), contactName.isCreateable()));
        return observed;
    }
    private static String classify(Boolean objectAllowed, Boolean fieldAllowed) {
        if (!objectAllowed) return 'object';
        if (!fieldAllowed) return 'field';
        return 'allowed';
    }
}
