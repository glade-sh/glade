@IsTest(SeeAllData=false)
private class DootLocaleU167A1031 {

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
    @IsTest static void tr() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('tr_TR','GMT','u1tr');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                String upper='I';String lower='i';String dotless=String.fromCharArray(new List<Integer>{305});String dotted=String.fromCharArray(new List<Integer>{304});stage='scalarEquality';o.put('equal',new List<Boolean>{upper==lower,upper==dotless,dotted==lower});o.put('controls',new List<Boolean>{upper.equalsIgnoreCase(lower),upper.equals(lower)});stage='ordinaryCollections';Map<String,Integer> m=new Map<String,Integer>{'I'=>1,'i'=>2};Set<String> xs=new Set<String>{'I','i'};o.put('collections',new List<Object>{m.size(),m.get(upper),m.get(lower),xs.size()});
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('U1.tr',o);
    }
    @IsTest static void en() {
        Map<String,Object> o=new Map<String,Object>(); String stage='fixtureInsert';
        try {
            User fixture=makeUser('en_US','GMT','u1en');
            stage='runAs';
            System.runAs(fixture) {
                o.put('context',context());
                String upper='I';String lower='i';String dotless=String.fromCharArray(new List<Integer>{305});String dotted=String.fromCharArray(new List<Integer>{304});stage='scalarEquality';o.put('equal',new List<Boolean>{upper==lower,upper==dotless,dotted==lower});o.put('controls',new List<Boolean>{upper.equalsIgnoreCase(lower),upper.equals(lower)});stage='ordinaryCollections';Map<String,Integer> m=new Map<String,Integer>{'I'=>1,'i'=>2};Set<String> xs=new Set<String>{'I','i'};o.put('collections',new List<Object>{m.size(),m.get(upper),m.get(lower),xs.size()});
                o.put('status','ok');
            }
        } catch(Exception e) { o.put('error',new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()}); }
        emit('U1.en',o);
    }
}
