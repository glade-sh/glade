@IsTest
private class LocaleFormatProbe62 {
    @IsTest
    static void observe() {
        User template = [
            SELECT ProfileId, TimeZoneSidKey, EmailEncodingKey, LanguageLocaleKey
            FROM User WHERE Id = :UserInfo.getUserId()
        ];
        List<String> localeKeys = new List<String>{'no', 'no_NO', 'nb_NO'};
        Map<String, Object> results = new Map<String, Object>();
        Integer suffix = 0;
        for (String localeKey : localeKeys) {
            User probe = new User(
                Alias = 'locprobe',
                Email = 'locale-probe@example.invalid',
                EmailEncodingKey = template.EmailEncodingKey,
                LastName = 'Synthetic Locale Probe',
                LanguageLocaleKey = template.LanguageLocaleKey,
                LocaleSidKey = localeKey,
                ProfileId = template.ProfileId,
                TimeZoneSidKey = template.TimeZoneSidKey,
                Username = 'locale.probe.62.' + UserInfo.getOrganizationId() + '.' + suffix + '@example.invalid'
            );
            suffix++;
            Database.SaveResult saved = Database.insert(probe, false);
            if (!saved.isSuccess()) {
                results.put(localeKey, 'rejected');
                continue;
            }
            User persisted = [SELECT Id, LocaleSidKey FROM User WHERE Id = :saved.getId()];
            System.runAs(persisted) {
                Date dateValue = Date.newInstance(2025, 11, 23);
                Datetime datetimeValue = Datetime.newInstance(2025, 11, 23, 14, 30, 0);
                results.put(localeKey, new Map<String, Object>{
                    'locale' => UserInfo.getLocale(),
                    'dateFormat' => dateValue.format(),
                    'datetimeFormat' => datetimeValue.format()
                });
            }
        }
        System.assert(false, 'OBS|dateFormatLocale|' + JSON.serialize(results));
    }
}
