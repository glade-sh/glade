@IsTest private class DthLocale {
    static User makeUser(String loc) {
        User u = new User(Alias='dthprobe', Email='probe'+'@'+'example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='en_US', LocaleSidKey=loc, TimeZoneSidKey='GMT', ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id, Username='probe'+String.valueOf(Crypto.getRandomLong())+'@example.invalid');
        insert u; return u;
    }
    static Map<String,Object> payload(String loc) {
        Datetime a = Datetime.newInstance(0L), b = Datetime.newInstance(1L);
        Date d = Date.newInstance(2026,11,16); Time t = Time.newInstance(12,34,56,789);
        List<Datetime> l = new List<Datetime>{a,b}; Set<Datetime> s = new Set<Datetime>{a,b}; Map<Datetime,Integer> m = new Map<Datetime,Integer>{a=>1,b=>2};
        return new Map<String,Object>{'locale'=>loc,'actual'=>UserInfo.getLocale(),'a_ms'=>a.getTime(),'a_hash'=>a.hashCode(),'b_ms'=>b.getTime(),'b_hash'=>b.hashCode(),'list_hash'=>l.hashCode(),'set_hash'=>s.hashCode(),'map_hash'=>m.hashCode(),'date_hash'=>d.hashCode(),'time_hash'=>t.hashCode()};
    }
    static void probe(String loc) {
        Map<String,Object> out = new Map<String,Object>{'requested'=>loc};
        try {
            User u = makeUser(loc);
            Datetime before = Datetime.newInstance(0L); Datetime inside;
            out.put('outside_locale',UserInfo.getLocale()); out.put('before_outside_hash',before.hashCode());
            System.runAs(u) { inside = Datetime.newInstance(0L); out.put('inside',payload(loc)); out.put('before_inside_hash',before.hashCode()); out.put('inside_inside_hash',inside.hashCode()); }
            out.put('inside_outside_hash',inside.hashCode());
        } catch (Exception e) { out.put('error',e.getTypeName()+': '+e.getMessage()); }
        System.assert(false,JSON.serialize(out));
    }
    @IsTest static void a_de(){probe('de');}
    @IsTest static void b_fr(){probe('fr');}
    @IsTest static void c_is(){probe('is_IS');}
    @IsTest static void d_mv(){probe('dv_MV');}
    @IsTest static void e_ir(){probe('fa_IR');}
    @IsTest static void f_il(){probe('iw_IL');}
    @IsTest static void g_he(){probe('he_IL');}
    @IsTest static void h_nl(){probe('nl_NL');}
    @IsTest static void i_tr(){probe('tr_TR');}
    @IsTest static void j_mx(){probe('es_MX');}
    @IsTest static void k_de(){probe('de_DE');}
}
