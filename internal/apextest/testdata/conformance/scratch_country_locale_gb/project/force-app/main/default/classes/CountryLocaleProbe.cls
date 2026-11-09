public class CountryLocaleProbe {
    static Object attempt(String label, Map<String,Object> sink, Callable body) {
        try { sink.put(label, body.call(label, null)); }
        catch (Exception issue) { sink.put(label, issue.getTypeName() + ':' + issue.getMessage()); }
        return null;
    }
    public class Fmt implements Callable {
        public Object call(String action, Map<String,Object> args) {
            Date day = Date.newInstance(2024, 12, 19);
            Datetime stamp = Datetime.newInstance(2024, 12, 19, 12, 49, 0);
            Datetime evening = Datetime.newInstance(2025, 11, 23, 14, 30, 0);
            if (action == 'dateFormat') return day.format();
            if (action == 'datetimeFormat') return stamp.format();
            if (action == 'eveningFormat') return evening.format();
            if (action == 'datetimeCustom') return evening.format('dd.MM.yyyy, \'kl\' HH:mm');
            if (action == 'datetimeMonthName') return stamp.format('MMMM EEEE');
            if (action == 'dateParse') return String.valueOf(Date.parse('19.12.2024'));
            if (action == 'dateParseSlash') return String.valueOf(Date.parse('12/19/2024'));
            if (action == 'datetimeParse') return String.valueOf(Datetime.parse('19.12.2024 12:49'));
            if (action == 'datetimeParseUs') return String.valueOf(Datetime.parse('12/19/2024 12:49 PM'));
            if (action == 'decimalFormat') return (1234567.891).format();
            if (action == 'integerFormat') return (1234567).format();
            if (action == 'decimalToString') return String.valueOf(1234.5);
            if (action == 'timeFormat') return String.valueOf(Time.newInstance(14, 30, 0, 0));
            if (action == 'locale') return UserInfo.getLocale();
            if (action == 'language') return UserInfo.getLanguage();
            if (action == 'timeZone') return UserInfo.getTimeZone().getID();
                        if (action == 'userRow') { User row = [SELECT LocaleSidKey, LanguageLocaleKey, TimeZoneSidKey, Country FROM User WHERE Id = :UserInfo.getUserId()]; return new Map<String,Object>{ 'LocaleSidKey' => row.LocaleSidKey, 'LanguageLocaleKey' => row.LanguageLocaleKey, 'TimeZoneSidKey' => row.TimeZoneSidKey, 'Country' => row.Country }; }
            if (action == 'orgRow') { Organization row = [SELECT DefaultLocaleSidKey, LanguageLocaleKey, TimeZoneSidKey, Country FROM Organization LIMIT 1]; return new Map<String,Object>{ 'DefaultLocaleSidKey' => row.DefaultLocaleSidKey, 'LanguageLocaleKey' => row.LanguageLocaleKey, 'TimeZoneSidKey' => row.TimeZoneSidKey, 'Country' => row.Country }; }
            if (action == 'formatGmt') return evening.formatGmt('dd.MM.yyyy HH:mm');
            if (action == 'dayName') return evening.format('EEEE');
            return null;
        }
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        for (String label : new List<String>{'dateFormat','datetimeFormat','eveningFormat','datetimeCustom','datetimeMonthName','dateParse','dateParseSlash','datetimeParse','datetimeParseUs','decimalFormat','integerFormat','decimalToString','timeFormat','locale','language','timeZone','userRow','orgRow','formatGmt','dayName'}) {
            attempt(label, result, new Fmt());
        }
        return result;
    }
}
