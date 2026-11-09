public class SnNull {
    public static String f(PageReference p) { return 'PageReference'; }
    public static String f(String s) { return 'String'; }
    public static String g(Integer i) { return 'Integer'; }
    public static String g(String s) { return 'String'; }
    public static String h(Auth.SessionLevel l) { return 'SessionLevel'; }
    public static String h(String s) { return 'String'; }
    public static String k(Object o) { return 'Object'; }

    public static List<String> rows = new List<String>();
    public static void report(String label, Object v, String pick) {
        rows.add(label + ' isnull=' + (v == null) + ' valueOf=' + String.valueOf(v) + ' pick=' + pick);
        System.debug('SNROW ' + label + ' isnull=' + (v == null) + ' valueOf=' + String.valueOf(v) + ' pick=' + pick);
    }

    public static void runSite() {
        try {
            PageReference p = Site.changePassword('Abcdef123!', 'Abcdef123!', 'Old12345!');
            report('site.local', p, f(p) + ' inst=' + (((Object) p) instanceof PageReference));
        } catch (Exception e) {
            System.debug('SNROW site.local EXC ' + e.getTypeName() + ': ' + e.getMessage()); rows.add('site.local EXC ' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            report('site.direct', Site.changePassword('Abcdef123!', 'Abcdef123!', 'Old12345!'), f(Site.changePassword('Abcdef123!', 'Abcdef123!', 'Old12345!')));
        } catch (Exception e) {
            System.debug('SNROW site.direct EXC ' + e.getTypeName() + ': ' + e.getMessage()); rows.add('site.direct EXC ' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            Object o = Site.changePassword('Abcdef123!', 'Abcdef123!');
            report('site.two', o, k(o) + ' inst=' + (o instanceof PageReference));
        } catch (Exception e) {
            System.debug('SNROW site.two EXC ' + e.getTypeName() + ': ' + e.getMessage()); rows.add('site.two EXC ' + e.getTypeName() + ': ' + e.getMessage());
        }
    }

    public static void runSession() {
        try {
            Auth.SessionLevel l = Auth.SessionManagement.getRequiredSessionLevelForProfile(null);
            report('session.local', l, h(l) + ' inst=' + (((Object) l) instanceof Auth.SessionLevel));
        } catch (Exception e) {
            System.debug('SNROW session.local EXC ' + e.getTypeName() + ': ' + e.getMessage()); rows.add('session.local EXC ' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            report('session.direct', Auth.SessionManagement.getRequiredSessionLevelForProfile(null), h(Auth.SessionManagement.getRequiredSessionLevelForProfile(null)));
        } catch (Exception e) {
            System.debug('SNROW session.direct EXC ' + e.getTypeName() + ': ' + e.getMessage()); rows.add('session.direct EXC ' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            Auth.SessionLevel l = Auth.SessionManagement.getRequiredSessionLevelForProfile(UserInfo.getProfileId());
            report('session.profile', l, h(l));
        } catch (Exception e) {
            System.debug('SNROW session.profile EXC ' + e.getTypeName() + ': ' + e.getMessage()); rows.add('session.profile EXC ' + e.getTypeName() + ': ' + e.getMessage());
        }
    }

    public static void sessionNull() { Auth.SessionLevel l = Auth.SessionManagement.getRequiredSessionLevelForProfile(null); report('sessionNull', l, h(l)); }
    public static void sessionProfile() { Auth.SessionLevel l = Auth.SessionManagement.getRequiredSessionLevelForProfile(UserInfo.getProfileId()); report('sessionProfile', l, h(l)); }
    public static void runAsyncOutside() {
        try {
            Integer d = AsyncInfo.getMinimumQueueableDelayInMinutes();
            report('async.outside', d, g(d));
        } catch (Exception e) {
            System.debug('SNROW async.outside EXC ' + e.getTypeName() + ': ' + e.getMessage()); rows.add('async.outside EXC ' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
