@IsTest
private class __CLASS__ {
    private static void observe(String locale, String language, Integer section) {
        User actor = new User(
            Alias='r473',
            Email='road473@example.invalid',
            EmailEncodingKey='UTF-8',
            LastName='Calendar probe',
            LanguageLocaleKey=language,
            LocaleSidKey=locale,
            ProfileId=UserInfo.getProfileId(),
            TimeZoneSidKey='GMT',
            Username='calendar.' + Crypto.getRandomLong() + '@example.invalid'
        );
        System.runAs(actor) {
            System.assertEquals(locale, UserInfo.getLocale());
            System.assertEquals(language, UserInfo.getLanguage());
            System.assertEquals('GMT', UserInfo.getTimeZone().getID());
__DATES__
            List<String> observations = new List<String>();
            for (Integer i = section * 8; i < (section + 1) * 8; i++) {
                Datetime d = dates[i];
                String localText = d.format(pattern);
                System.assertEquals(localText, d.format(pattern, 'GMT'), 'explicit ' + i);
                System.assertEquals(localText, d.formatGmt(pattern), 'GMT ' + i);
                observations.add(localText);
            }
            System.assert(false, 'RD473_ROWS=' + String.join(observations, ';'));
        }
    }
__METHODS__
}
