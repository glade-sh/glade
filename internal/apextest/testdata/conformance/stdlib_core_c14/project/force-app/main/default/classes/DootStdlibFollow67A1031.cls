@IsTest(SeeAllData=false)
private class DootStdlibFollow67A1031 {

    private static Map<String,Object> err(String stage, Exception e) {
        return new Map<String,Object>{'stage'=>stage,'et'=>e.getTypeName(),'em'=>e.getMessage()};
    }
    private static void emit(String id, Map<String,Object> o) {
        String j=JSON.serialize(o);
        if(j.length()>600) j=JSON.serialize(new Map<String,Object>{'status'=>'inconclusive_overlength','originalLength'=>j.length()});
        System.assert(false,'CAPTURE|'+id+'|'+j);
    }
    private static String chars(List<Integer> u) { return String.fromCharArray(u); }
    private static Map<String,Object> units(String s) {
        return new Map<String,Object>{'n'=>s.length(),'u'=>s.getChars()};
    }
    @IsTest static void baseline() { System.assertEquals(2,1+1,'BASELINE|arithmetic'); }
    @IsTest static void environment() {
        emit('ENV',new Map<String,Object>{'api'=>'67.0','route'=>'named-test-check-only','selectorNs'=>'gladens001','locale'=>UserInfo.getLocale(),'language'=>UserInfo.getLanguage(),'tz'=>UserInfo.getTimeZone().getID()});
    }
    @IsTest static void a1Title() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='swapCase';o.put('out',units(chars(new List<Integer>{453}).swapCase()));o.put('ascii',units('Ab'.swapCase())); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A1.title',o);
    }
    @IsTest static void a2Surrogate() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { String s=chars(new List<Integer>{55357,56832,97,98,99});stage='abbreviate';o.put('in',units(s));o.put('out',units(s.abbreviate(4,0))); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A2.surrogate',o);
    }
    @IsTest static void a2Ascii() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='abbreviate';o.put('out',units('abcdef'.abbreviate(4,0))); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A2.ascii',o);
    }
    @IsTest static void a3negempty() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',-1);o.put('out',''.split('',-1)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.neg.empty',o);
    }
    @IsTest static void a3neggroup() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',-1);o.put('out',''.split('(?:)',-1)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.neg.group',o);
    }
    @IsTest static void a3zeroempty() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',0);o.put('out',''.split('',0)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.zero.empty',o);
    }
    @IsTest static void a3zerogroup() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',0);o.put('out',''.split('(?:)',0)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.zero.group',o);
    }
    @IsTest static void a3oneempty() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',1);o.put('out',''.split('',1)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.one.empty',o);
    }
    @IsTest static void a3onegroup() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',1);o.put('out',''.split('(?:)',1)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.one.group',o);
    }
    @IsTest static void a3twoempty() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',2);o.put('out',''.split('',2)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.two.empty',o);
    }
    @IsTest static void a3twogroup() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('limit',2);o.put('out',''.split('(?:)',2)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.two.group',o);
    }
    @IsTest static void a3Nonempty() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='split';o.put('out','a,b,'.split(',',-1)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A3.nonempty',o);
    }
    @IsTest static void a4sigma() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { String s=chars(new List<Integer>{927,931});stage='lower';o.put('in',units(s));o.put('out',units(s.toLowerCase())); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A4.sigma',o);
    }
    @IsTest static void a4dotted() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { String s=chars(new List<Integer>{304});stage='lower';o.put('in',units(s));o.put('out',units(s.toLowerCase())); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A4.dotted',o);
    }
    @IsTest static void a4decomp() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { String s=chars(new List<Integer>{73,775});stage='lower';o.put('in',units(s));o.put('out',units(s.toLowerCase())); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A4.decomp',o);
    }
    private static String decimalInput(Integer zeros) { String s='1';for(Integer i=0;i<zeros;i++) s+='0';return s; }
    @IsTest static void a5Admission() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='valueOf';Decimal d=Decimal.valueOf('123'+decimalInput(38).substring(1));o.put('precision',d.precision());o.put('scale',d.scale());o.put('plain',d.toPlainString()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A5.admit',o);
    }
    @IsTest static void a5Slash() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='valueOf';Decimal d=Decimal.valueOf('123'+decimalInput(38).substring(1));stage='slash';Decimal q=d/1;o.put('equal',q==d);o.put('plain',q.toPlainString());o.put('precision',q.precision());o.put('scale',q.scale()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A5.slash',o);
    }
    @IsTest static void a5Divide() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='valueOf';Decimal d=Decimal.valueOf('123'+decimalInput(38).substring(1));stage='divide';Decimal q=d.divide(1,0);o.put('equal',q==d);o.put('plain',q.toPlainString()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A5.divide',o);
    }
    @IsTest static void a5Normal() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='division';Decimal d=12345;Decimal q=d/1;o.put('equal',q==d);o.put('plain',q.toPlainString()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A5.normal',o);
    }
    @IsTest static void a7Magnitude() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { String s=decimalInput(309);o.put('digits',s.length());stage='valueOf';Decimal d=Decimal.valueOf(s);o.put('precision',d.precision());o.put('scale',d.scale()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A7.magnitude',o);
    }
    @IsTest static void a10singapore() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='format';o.put('out',Datetime.newInstanceGmt(2026,1,1,0,0,0).format('yyyy-MM-dd HH:mm Z','Asia/Singapore')); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A10.singapore',o);
    }
    @IsTest static void a10gmt() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='format';o.put('out',Datetime.newInstanceGmt(2026,1,1,0,0,0).format('yyyy-MM-dd HH:mm Z','GMT')); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A10.gmt',o);
    }
    @IsTest static void a10losangeles() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='format';o.put('out',Datetime.newInstanceGmt(2026,1,1,0,0,0).format('yyyy-MM-dd HH:mm Z','America/Los_Angeles')); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A10.losangeles',o);
    }
    @IsTest static void a11Padding() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='formatGmt';o.put('out',Datetime.newInstanceGmt(2026,1,6,0,0,0).formatGmt('F|FFF'));o.put('dayControl',Datetime.newInstanceGmt(2026,1,6,0,0,0).formatGmt('d|ddd')); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A11.padding',o);
    }
    @IsTest static void a13future() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='newInstanceGmt';Datetime d=Datetime.newInstanceGmt(2400,1,1,0,0,0);stage='getTime';Long n=d.getTime();o.put('ms',n);stage='roundtrip';o.put('out',Datetime.newInstance(n).formatGmt('yyyy-MM-dd HH:mm:ss')); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A13.future',o);
    }
    @IsTest static void a13ordinary() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='newInstanceGmt';Datetime d=Datetime.newInstanceGmt(2026,1,1,0,0,0);stage='getTime';Long n=d.getTime();o.put('ms',n);stage='roundtrip';o.put('out',Datetime.newInstance(n).formatGmt('yyyy-MM-dd HH:mm:ss')); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A13.ordinary',o);
    }
    @IsTest static void a15Mutation() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Account k=new Account(Name='before');Map<Account,Integer> m=new Map<Account,Integer>();stage='put';m.put(k,7);o.put('before',new List<Object>{m.containsKey(k),m.get(k)});stage='mutate';k.Name='after';o.put('after',new List<Object>{m.containsKey(k),m.get(k)});o.put('size',m.size()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A15.mutate',o);
    }
    @IsTest static void a15Unchanged() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Account k=new Account(Name='before');Map<Account,Integer> m=new Map<Account,Integer>();stage='put';m.put(k,7);o.put('out',new List<Object>{m.containsKey(k),m.get(k)}); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A15.unchanged',o);
    }
    private class Duck { public Integer compareTo(Object other) { return 0; } }
    private class ComparableDuck implements Comparable { public Integer compareTo(Object other) { return 0; } }
    @IsTest static void a18Duck() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { List<Duck> xs=new List<Duck>{new Duck(),new Duck()};stage='sort';xs.sort();o.put('size',xs.size()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A18.duck',o);
    }
    @IsTest static void a18Comparable() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { List<ComparableDuck> xs=new List<ComparableDuck>{new ComparableDuck(),new ComparableDuck()};stage='sort';xs.sort();o.put('size',xs.size()); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A18.comparable',o);
    }
    @IsTest static void a19Collection() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { Map<List<String>,Integer> m=new Map<List<String>,Integer>();m.put(new List<String>{'x'},1);stage='serialize';o.put('out',JSON.serialize(m)); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A19.collection',o);
    }
    @IsTest static void a19Empty() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='serialize';o.put('out',JSON.serialize(new Map<List<String>,Integer>())); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A19.empty',o);
    }
    @IsTest static void a19String() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { stage='serialize';o.put('out',JSON.serialize(new Map<String,Integer>{'x'=>1})); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('A19.string',o);
    }
    @IsTest static void u4BmpBoundary() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { String s=chars(new List<Integer>{55296,56320});String b=chars(new List<Integer>{57344});stage='compareTo';o.put('cmp',s.compareTo(b));o.put('reverseCmp',b.compareTo(s));List<String> xs=new List<String>{b,s};stage='sort';xs.sort();o.put('sorted',new List<Object>{units(xs[0]),units(xs[1])}); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('U4.boundary',o);
    }
    @IsTest static void u4Ascii() {
        Map<String,Object> o=new Map<String,Object>(); String stage='begin';
        try { List<String> xs=new List<String>{'b','a'};stage='sort';xs.sort();o.put('sorted',xs);o.put('cmp','a'.compareTo('b')); o.put('status','ok'); } catch(Exception e) { o.put('error',err(stage,e)); }
        emit('U4.ascii',o);
    }
}
