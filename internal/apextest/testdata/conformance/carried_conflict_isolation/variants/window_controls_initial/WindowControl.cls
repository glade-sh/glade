public class WindowControl {
    public static Object observe() {
        Map<String,Object> out = new Map<String,Object>();
        List<Profile> ps=[SELECT Id FROM Profile ORDER BY Name];
        List<Profile> pl=[SELECT Id FROM Profile ORDER BY Name LIMIT 1];
        Map<String,Object> profileControls = new Map<String,Object>{'count'=>ps.size(),'limitEqualsSorted'=>pl[0].Id==ps[0].Id};
        for (Integer n=1; n<=8; n++) {
            List<Profile> selected=Database.query('SELECT Id FROM Profile ORDER BY Name LIMIT '+n);
            Boolean same=true;
            for (Integer i=0; i<selected.size(); i++) if (selected[i].Id != ps[i].Id) same=false;
            profileControls.put('limit'+n+'EqualsSortedPrefix',same);
        }
        List<Profile> pd=[SELECT Id FROM Profile ORDER BY Name DESC];
        List<Profile> pdl=[SELECT Id FROM Profile ORDER BY Name DESC LIMIT 1];
        profileControls.put('descendingLimitEqualsSorted',pd[0].Id==pdl[0].Id);
        List<Profile> pn=[SELECT Id FROM Profile WHERE Name != null ORDER BY Name];
        List<Profile> pnl=[SELECT Id FROM Profile WHERE Name != null ORDER BY Name LIMIT 1];
        profileControls.put('nonNullLimitEqualsSorted',pn[0].Id==pnl[0].Id);
        out.put('profile', profileControls);
        List<UserLicense> us=[SELECT Id FROM UserLicense ORDER BY Name];
        List<UserLicense> ul=[SELECT Id FROM UserLicense ORDER BY Name LIMIT 1];
        out.put('userLicense', new Map<String,Object>{'count'=>us.size(),'limitEqualsSorted'=>ul[0].Id==us[0].Id});
        List<PermissionSet> ss=[SELECT Id FROM PermissionSet ORDER BY Name];
        List<PermissionSet> sl=[SELECT Id FROM PermissionSet ORDER BY Name LIMIT 1];
        out.put('permissionSet', new Map<String,Object>{'count'=>ss.size(),'limitEqualsSorted'=>sl[0].Id==ss[0].Id});
        List<EmailServicesFunction> fs=[SELECT Id FROM EmailServicesFunction ORDER BY FunctionName];
        List<EmailServicesFunction> fl=[SELECT Id FROM EmailServicesFunction ORDER BY FunctionName LIMIT 1];
        out.put('emailFunction', new Map<String,Object>{'count'=>fs.size(),'limitEqualsSorted'=>fl[0].Id==fs[0].Id});
        List<EmailServicesAddress> es=[SELECT Id FROM EmailServicesAddress ORDER BY DeveloperName];
        List<EmailServicesAddress> el=[SELECT Id FROM EmailServicesAddress ORDER BY DeveloperName LIMIT 1];
        out.put('emailAddress', new Map<String,Object>{'count'=>es.size(),'limitEqualsSorted'=>el[0].Id==es[0].Id});
        Savepoint s=Database.setSavepoint();
        insert new List<Account>{new Account(Name='Window z'),new Account(Name='Window a'),new Account(Name='Window m')};
        List<Account> ac=[SELECT Id FROM Account WHERE Name LIKE 'Window %' ORDER BY Name];
        List<Account> al=[SELECT Id FROM Account WHERE Name LIKE 'Window %' ORDER BY Name LIMIT 1];
        out.put('account', new Map<String,Object>{'count'=>ac.size(),'limitEqualsSorted'=>al[0].Id==ac[0].Id});
        Database.rollback(s);
        return out;
    }
}
