@IsTest private class CalendarSettingsObservation {
    static void observe() {
        Datetime now = System.now();
        Date today = Date.today();
        Datetime fixed = Datetime.valueOfGmt('2021-08-11 00:00:00');
        Datetime winter = Datetime.valueOfGmt('2026-01-15 00:30:00');
        Datetime summer = Datetime.valueOfGmt('2026-07-15 00:30:00');
        insert as system new List<CalendarObservation__c>{
            new CalendarObservation__c(Name='Current', Day__c=today, Moment__c=now),
            new CalendarObservation__c(Name='Tomorrow', Day__c=today.addDays(1), Moment__c=now.addDays(1)),
            new CalendarObservation__c(Name='Following', Day__c=today.addDays(7), Moment__c=now.addDays(7))
        };
        CalendarObservation__c record = [SELECT Today__c, Now__c FROM CalendarObservation__c WHERE Name='Current' WITH SYSTEM_MODE];
        List<String> row = new List<String>{
            now.formatGmt('yyyy-MM-dd HH:mm:ss'), UserInfo.getTimeZone().getID(), UserInfo.getLocale(), UserInfo.getLanguage(), UserInfo.getDefaultCurrency(),
            String.valueOf(fixed.date()), String.valueOf(fixed.dateGmt()), String.valueOf(fixed.time()),
            winter.format('yyyy-MM-dd HH:mm Z'), summer.format('yyyy-MM-dd HH:mm Z'), summer.format(),
            String.valueOf(today), String.valueOf(record.Today__c), String.valueOf(Math.abs(record.Now__c.getTime()-now.getTime()) < 5000),
            String.valueOf([SELECT COUNT() FROM CalendarObservation__c WHERE Day__c=TODAY WITH SYSTEM_MODE]),
            String.valueOf([SELECT COUNT() FROM CalendarObservation__c WHERE Moment__c=TODAY WITH SYSTEM_MODE]),
            String.valueOf([SELECT COUNT() FROM CalendarObservation__c WHERE Day__c=THIS_WEEK WITH SYSTEM_MODE]),
            String.valueOf([SELECT COUNT() FROM CalendarObservation__c WHERE Day__c=NEXT_WEEK WITH SYSTEM_MODE]),
            String.valueOf([SELECT COUNT() FROM CalendarObservation__c WHERE Moment__c=THIS_WEEK WITH SYSTEM_MODE]),
            String.valueOf([SELECT COUNT() FROM CalendarObservation__c WHERE Moment__c=NEXT_WEEK WITH SYSTEM_MODE])
        };
        System.assert(false,'CALENDAR_ROW|'+JSON.serialize(row));
    }
    static User user(String zone) {
        return new User(Alias='caluser',Email='calendar@example.test',EmailEncodingKey='UTF-8',LastName='Calendar',LanguageLocaleKey='en_US',LocaleSidKey='en_GB',ProfileId=UserInfo.getProfileId(),TimeZoneSidKey=zone,Username='calendar.'+zone.replace('/','.')+'.'+System.now().getTime()+'@example.test');
    }
    @IsTest static void berlin() { System.runAs(user('Europe/Berlin')) { observe(); } }
    @IsTest static void german() { User u=user('America/New_York'); u.LocaleSidKey='de_DE'; u.LanguageLocaleKey='de'; System.runAs(u) { observe(); } }
    @IsTest static void utc() { System.runAs(user('GMT')) { observe(); } }
    @IsTest static void adminDefaults() {
        Organization org = [SELECT TimeZoneSidKey, DefaultLocaleSidKey, LanguageLocaleKey FROM Organization];
        System.assert(false,'CALENDAR_ROW|'+JSON.serialize(new List<String>{UserInfo.getTimeZone().getID(),org.TimeZoneSidKey,UserInfo.getLocale(),org.DefaultLocaleSidKey,UserInfo.getLanguage(),org.LanguageLocaleKey}));
    }
}
