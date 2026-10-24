public with sharing class MeadowSetupProbe implements Schedulable {
    public void execute(SchedulableContext context) {}
    private static Set<Id> roleIds;
    private static Id groupId, permissionId, assignmentId, scheduleId, jobId, adminProfileId;
    public static Map<String, Object> observe() {
        List<UserRole> roles = new List<UserRole>{
            new UserRole(Name='Meadow One', DeveloperName='MeadowOne'),
            new UserRole(Name='Meadow Two', DeveloperName='MeadowTwo'),
            new UserRole(Name='Meadow Three', DeveloperName='MeadowThree')
        };
        insert roles;
        roleIds = new Map<Id, UserRole>(roles).keySet();
        Group regular = new Group(Name='Meadow Readers', DeveloperName='MeadowReaders', Type='Regular');
        insert regular;
        groupId = regular.Id;
        PermissionSet permission = new PermissionSet(Name='MeadowRead', Label='Meadow Read');
        insert permission;
        permissionId = permission.Id;
        PermissionSetAssignment assignment = new PermissionSetAssignment(PermissionSetId=permissionId, AssigneeId=UserInfo.getUserId());
        insert assignment;
        assignmentId = assignment.Id;
        adminProfileId = UserInfo.getProfileId();
        scheduleId = System.schedule('Meadow Setup Read', '0 0 0 1 1 ? 2099', new MeadowSetupProbe());
        jobId = System.enqueueJob(new MeadowSetupJob());
        Map<String, Object> result = new Map<String, Object>();
        snapshot(result, 'admin');
        if (Test.isRunningTest()) {
            for (String profileName : new List<String>{'Standard User', 'Minimum Access - Salesforce'}) {
                Profile profile = [SELECT Id FROM Profile WHERE Name=:profileName WITH SYSTEM_MODE LIMIT 1];
                User reader = new User(ProfileId=profile.Id, Username='meadow.'+profile.Id+'@example.invalid',
                    Email='meadow@example.invalid', LastName='Reader', Alias='meadow', TimeZoneSidKey='America/Los_Angeles',
                    LocaleSidKey='en_US', EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US');
                System.runAs(reader) { snapshot(result, profileName); }
            }
        }
        return result;
    }
    private static void snapshot(Map<String, Object> result, String route) {
        Set<Id> selectedRoles = roleIds;
        List<String> names = new List<String>();
        List<UserRole> visibleRoles = [SELECT DeveloperName FROM UserRole WHERE DeveloperName IN ('MeadowOne','MeadowTwo','MeadowThree') WITH SYSTEM_MODE ORDER BY DeveloperName];
        for (UserRole role : visibleRoles) names.add(role.DeveloperName);
        result.put(route+'.roles.list', names);
        result.put(route+'.roles.count', [SELECT COUNT() FROM UserRole WHERE Id IN :selectedRoles WITH SYSTEM_MODE]);
        result.put(route+'.roles.dynamic', Database.query('SELECT Id FROM UserRole WHERE Id IN :selectedRoles', AccessLevel.SYSTEM_MODE).size());
        List<AggregateResult> grouped = [SELECT DeveloperName n, COUNT(Id) c FROM UserRole WHERE Id IN :selectedRoles WITH SYSTEM_MODE GROUP BY DeveloperName];
        result.put(route+'.roles.grouped', grouped.size());
        Map<String, Object> binds = new Map<String, Object>{'ids'=>roleIds};
        result.put(route+'.roles.binds', Database.queryWithBinds('SELECT Id FROM UserRole WHERE Id IN :ids', binds, AccessLevel.SYSTEM_MODE).size());
        result.put(route+'.roles.bindCount', Database.countQueryWithBinds('SELECT COUNT() FROM UserRole WHERE Id IN :ids', binds, AccessLevel.SYSTEM_MODE));
        try { result.put(route+'.roles.user', [SELECT COUNT() FROM UserRole WHERE Id IN :selectedRoles WITH USER_MODE]); }
        catch (Exception e) { result.put(route+'.roles.user', 'denied'); }
        result.put(route+'.roles.accessible', Schema.SObjectType.UserRole.isAccessible());
        result.put(route+'.group.regular', [SELECT COUNT() FROM Group WHERE Id=:groupId WITH SYSTEM_MODE]);
        result.put(route+'.group.internal', [SELECT COUNT() FROM Group WHERE DeveloperName='AllInternalUsers' AND Type='Organization' WITH SYSTEM_MODE]);
        result.put(route+'.group.customer', [SELECT COUNT() FROM Group WHERE DeveloperName='AllCustomerPortalUsers' WITH SYSTEM_MODE]);
        result.put(route+'.group.partner', [SELECT COUNT() FROM Group WHERE DeveloperName='AllPartnerUsers' WITH SYSTEM_MODE]);
        result.put(route+'.profile', [SELECT COUNT() FROM Profile WHERE Id=:adminProfileId WITH SYSTEM_MODE]);
        result.put(route+'.permission', [SELECT COUNT() FROM PermissionSet WHERE Id=:permissionId WITH SYSTEM_MODE]);
        result.put(route+'.assignment', [SELECT COUNT() FROM PermissionSetAssignment WHERE Id=:assignmentId WITH SYSTEM_MODE]);
        result.put(route+'.recordtype', [SELECT COUNT() FROM RecordType WHERE DeveloperName='MeadowRecord' AND SObjectType='Account' WITH SYSTEM_MODE]);
        result.put(route+'.schedule.detail', [SELECT COUNT() FROM CronJobDetail WHERE Name='Meadow Setup Read' WITH SYSTEM_MODE]);
        result.put(route+'.schedule', [SELECT COUNT() FROM CronTrigger WHERE Id=:scheduleId WITH SYSTEM_MODE]);
        result.put(route+'.job', [SELECT COUNT() FROM AsyncApexJob WHERE Id=:jobId WITH SYSTEM_MODE]);
    }
}
