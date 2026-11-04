@IsTest(SeeAllData=false)
private class DootLocaleA1267A1031 {

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
    @IsTest static void fr() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('fr_FR','GMT','a12fr');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                Date d=Date.newInstance(2008,1,28);o.put('input',dateUnits(d));stage='format';String s=d.format();o.put('formatted',s);stage='parse';Date parsed=Date.parse(s);o.put('parsed',dateUnits(parsed));
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('A12.fr',o);
    }
    @IsTest static void en() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('en_US','GMT','a12en');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                Date d=Date.newInstance(2008,1,28);o.put('input',dateUnits(d));stage='format';String s=d.format();o.put('formatted',s);stage='parse';Date parsed=Date.parse(s);o.put('parsed',dateUnits(parsed));
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('A12.en',o);
    }
}
