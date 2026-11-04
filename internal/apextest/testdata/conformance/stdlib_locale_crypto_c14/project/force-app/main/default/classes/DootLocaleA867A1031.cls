@IsTest(SeeAllData=false)
private class DootLocaleA867A1031 {

    private static User makeUser(String locale, String zone, String tag) {
        // New synthetic setup record only. No lookup/update of any existing User.
        User u=new User(
            Alias='dcap', Email='capture@example.invalid', EmailEncodingKey='UTF-8',
            LastName='SyntheticCapture', LanguageLocaleKey='en_US',
            LocaleSidKey=locale, TimeZoneSidKey=zone,
            ProfileId=UserInfo.getProfileId(),
            Username='dc.'+UserInfo.getOrganizationId()+'.'+String.valueOf(Datetime.now().getTime())+'.'+tag+'@example.invalid'
        );
        insert u;
        return u;
    }
    private static Map<String,Object> context() {
        return new Map<String,Object>{'locale'=>UserInfo.getLocale(),'language'=>UserInfo.getLanguage(),'tz'=>UserInfo.getTimeZone().getID()};
    }
    private static List<Integer> dateUnits(Date d) { return new List<Integer>{d.year(),d.month(),d.day()}; }
    private static void emit(String id, Map<String,Object> o) {
        String j=JSON.serialize(o);
        if(j.length()>600) j=JSON.serialize(new Map<String,Object>{'status'=>'inconclusive_overlength','originalLength'=>j.length()});
        System.assert(false,'CAPTURE|'+id+'|'+j);
    }
    @IsTest static void baseline() { System.assertEquals(2,1+1,'BASELINE|arithmetic'); }
    @IsTest static void lapair() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('en_US','America/Los_Angeles','lapair');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                Datetime a=Datetime.newInstanceGmt(2026,1,1,23,30,0);Datetime b=Datetime.newInstanceGmt(2026,1,2,0,30,0);stage='isSameDay';o.put('same',a.isSameDay(b));o.put('reverse',b.isSameDay(a));o.put('localDates',new List<Object>{dateUnits(a.date()),dateUnits(b.date())});o.put('gmtDates',new List<Object>{dateUnits(a.dateGmt()),dateUnits(b.dateGmt())});o.put('sameInstant',a.isSameDay(a));
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('A8.lapair',o);
    }
    @IsTest static void larev() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('en_US','America/Los_Angeles','larev');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                Datetime a=Datetime.newInstanceGmt(2026,1,2,0,30,0);Datetime b=Datetime.newInstanceGmt(2026,1,2,8,30,0);stage='isSameDay';o.put('same',a.isSameDay(b));o.put('reverse',b.isSameDay(a));o.put('localDates',new List<Object>{dateUnits(a.date()),dateUnits(b.date())});o.put('gmtDates',new List<Object>{dateUnits(a.dateGmt()),dateUnits(b.dateGmt())});o.put('sameInstant',a.isSameDay(a));
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('A8.larev',o);
    }
    @IsTest static void gmtpair() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('en_US','GMT','gmtpair');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                Datetime a=Datetime.newInstanceGmt(2026,1,1,23,30,0);Datetime b=Datetime.newInstanceGmt(2026,1,2,0,30,0);stage='isSameDay';o.put('same',a.isSameDay(b));o.put('reverse',b.isSameDay(a));o.put('localDates',new List<Object>{dateUnits(a.date()),dateUnits(b.date())});o.put('gmtDates',new List<Object>{dateUnits(a.dateGmt()),dateUnits(b.dateGmt())});o.put('sameInstant',a.isSameDay(a));
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('A8.gmtpair',o);
    }
    @IsTest static void gmtrev() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('en_US','GMT','gmtrev');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                Datetime a=Datetime.newInstanceGmt(2026,1,2,0,30,0);Datetime b=Datetime.newInstanceGmt(2026,1,2,8,30,0);stage='isSameDay';o.put('same',a.isSameDay(b));o.put('reverse',b.isSameDay(a));o.put('localDates',new List<Object>{dateUnits(a.date()),dateUnits(b.date())});o.put('gmtDates',new List<Object>{dateUnits(a.dateGmt()),dateUnits(b.dateGmt())});o.put('sameInstant',a.isSameDay(a));
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('A8.gmtrev',o);
    }
}
