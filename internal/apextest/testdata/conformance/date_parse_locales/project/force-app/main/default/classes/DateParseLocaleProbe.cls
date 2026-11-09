public class DateParseLocaleProbe {
    static final List<String> DATE_TEXTS = new List<String>{'19.12.2024','19/12/2024','12/19/2024','2024-12-19','2024-12-19 10:11:12','garbage'};
    static final List<String> DATETIME_TEXTS = new List<String>{'19.12.2024 12:49','19/12/2024 12:49','12/19/2024 12:49 PM','2024-12-19 12:49:00','2024-12-19T12:49:00Z','garbage'};
    static String fail(Exception issue, String text) {
        String type = issue.getTypeName() == 'System.TypeException' ? '' : issue.getTypeName() + ':';
        return 'E:' + type + issue.getMessage().replace(text, '#');
    }
    public static List<String> observeCurrent() {
        List<String> result = new List<String>{UserInfo.getLocale()};
        for (String text : DATE_TEXTS) {
            try { Date d = Date.parse(text); result.add('Date.parse|' + text + '=' + String.valueOf(d)); } catch (Exception issue) { result.add('Date.parse|' + text + '=' + fail(issue, text)); }
            try { Date d = Date.valueOf(text); result.add('Date.valueOf|' + text + '=' + String.valueOf(d)); } catch (Exception issue) { result.add('Date.valueOf|' + text + '=' + fail(issue, text)); }
        }
        for (String text : DATETIME_TEXTS) {
            try { Datetime d = Datetime.parse(text); result.add('Datetime.parse|' + text + '=' + String.valueOf(d)); } catch (Exception issue) { result.add('Datetime.parse|' + text + '=' + fail(issue, text)); }
            try { Datetime d = Datetime.valueOf(text); result.add('Datetime.valueOf|' + text + '=' + String.valueOf(d)); } catch (Exception issue) { result.add('Datetime.valueOf|' + text + '=' + fail(issue, text)); }
        }
        return result;
    }
    public static Map<String,Object> observeLocales(List<String> locales) {
        Map<String,Object> all = new Map<String,Object>();
        Profile profile = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        Integer n = 0;
        for (String locale : locales) {
            n++;
            User u = new User(Alias = 'dp' + n, Email = 'dp' + n + '@example.com', EmailEncodingKey = 'UTF-8', LastName = 'Probe' + n,
                LanguageLocaleKey = 'en_US', LocaleSidKey = locale, ProfileId = profile.Id, TimeZoneSidKey = 'Europe/Berlin',
                Username = 'dp' + n + System.currentTimeMillis() + '@example.com');
            insert u;
            System.runAs(u) { all.put(locale, observeCurrent()); }
        }
        return all;
    }
}
