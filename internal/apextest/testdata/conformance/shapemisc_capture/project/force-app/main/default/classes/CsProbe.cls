public class CsProbe {
    static String ex(Exception e) { return e.getTypeName() + '|' + e.getMessage() + '|' + e.getLineNumber(); }
    public static List<String> run() {
        List<String> out = new List<String>();
        CsHolder h = new CsHolder();
        CsSub sub = new CsSub();
        try { String x = h.lst.Text__c; out.add('b01_chainUnsetList=' + x); } catch (Exception e) { out.add('b01_chainUnsetList=EX ' + ex(e)); }
        try { out.add('b02_implicitUnsetList=' + h.readImplicit()); } catch (Exception e) { out.add('b02_implicitUnsetList=EX ' + ex(e)); }
        try { out.add('b03_thisUnsetList=' + h.readThis()); } catch (Exception e) { out.add('b03_thisUnsetList=EX ' + ex(e)); }
        try { out.add('b04_overrideImplicit=' + sub.readImplicit()); } catch (Exception e) { out.add('b04_overrideImplicit=EX ' + ex(e)); }
        try { out.add('b05_superCall=' + sub.readSuper()); } catch (Exception e) { out.add('b05_superCall=EX ' + ex(e)); }
        CsHolder h6 = new CsHolder();
        h6.lst = ProbeList__c.getValues('absent');
        try { String x = h6.lst.Text__c; out.add('b06_assignedNullGetValues=' + x); } catch (Exception e) { out.add('b06_assignedNullGetValues=EX ' + ex(e)); }
        try { out.add('b07_assignedNullImplicit=' + h6.readImplicit()); } catch (Exception e) { out.add('b07_assignedNullImplicit=EX ' + ex(e)); }
        CsHolder h8 = new CsHolder();
        h8.lst = ProbeList__c.getInstance('absent');
        try { String x = h8.lst.Text__c; out.add('b08_assignedNullGetInstance=' + x); } catch (Exception e) { out.add('b08_assignedNullGetInstance=EX ' + ex(e)); }
        try { ProbeList__c s = h.lst; String x = s.Text__c; out.add('b09_localFromUnsetMember=' + x); } catch (Exception e) { out.add('b09_localFromUnsetMember=EX ' + ex(e)); }
        try { String x = h.hier.Text__c; out.add('b10_chainUnsetHier=' + x); } catch (Exception e) { out.add('b10_chainUnsetHier=EX ' + ex(e)); }
        try { out.add('b11_implicitUnsetHier=' + h.readHier()); } catch (Exception e) { out.add('b11_implicitUnsetHier=EX ' + ex(e)); }
        CsHolder h12 = new CsHolder();
        h12.hier = ProbeHier__c.getValues(UserInfo.getUserId());
        out.add('b12_hierGetValuesIsNull=' + (h12.hier == null));
        try { String x = h12.hier.Text__c; out.add('b12_hierGetValuesRead=' + x); } catch (Exception e) { out.add('b12_hierGetValuesRead=EX ' + ex(e)); }
        try { out.add('b12_hierGetValuesImplicit=' + h12.readHier()); } catch (Exception e) { out.add('b12_hierGetValuesImplicit=EX ' + ex(e)); }
        CsHolder h13 = new CsHolder();
        h13.hier = ProbeHier__c.getInstance();
        out.add('b13_hierGetInstanceIsNull=' + (h13.hier == null));
        try { String x = h13.hier.Text__c; out.add('b13_hierGetInstanceRead=' + x); } catch (Exception e) { out.add('b13_hierGetInstanceRead=EX ' + ex(e)); }
        try { String x = ProbeHier__c.getOrgDefaults().Text__c; out.add('b14_hierOrgDefaults=' + x); } catch (Exception e) { out.add('b14_hierOrgDefaults=EX ' + ex(e)); }
        try { String x = CsHolder.slst.Text__c; out.add('b15_staticUnsetChain=' + x); } catch (Exception e) { out.add('b15_staticUnsetChain=EX ' + ex(e)); }
        try { out.add('b16_staticUnsetImplicit=' + CsHolder.readStatic()); } catch (Exception e) { out.add('b16_staticUnsetImplicit=EX ' + ex(e)); }
        try { Boolean b = h.lst.Text__c == null; out.add('b17_compareNull=' + b); } catch (Exception e) { out.add('b17_compareNull=EX ' + ex(e)); }
        try { String x = 'a' + h.lst.Text__c; out.add('b18_concat=' + x); } catch (Exception e) { out.add('b18_concat=EX ' + ex(e)); }
        try { Boolean b = String.isBlank(h.lst.Text__c); out.add('b19_argIsBlank=' + b); } catch (Exception e) { out.add('b19_argIsBlank=EX ' + ex(e)); }
        try { h.lst.Text__c = 'x'; out.add('b20_write=ok'); } catch (Exception e) { out.add('b20_write=EX ' + ex(e)); }
        try { Object o = h.lst.Text__c; out.add('b21_objectVar=' + o); } catch (Exception e) { out.add('b21_objectVar=EX ' + ex(e)); }
        try { Object o = h.lst.get('Text__c'); out.add('b22_dynamicGet=' + o); } catch (Exception e) { out.add('b22_dynamicGet=EX ' + ex(e)); }
        try { Object o = h.lst.Id; out.add('b23_idRead=' + o); } catch (Exception e) { out.add('b23_idRead=EX ' + ex(e)); }
        try { Object o = h.lst.Name; out.add('b24_nameRead=' + o); } catch (Exception e) { out.add('b24_nameRead=EX ' + ex(e)); }
        ProbeList__c loc = null;
        try { String x = loc.Text__c; out.add('b25_nullLocal=' + x); } catch (Exception e) { out.add('b25_nullLocal=EX ' + ex(e)); }
        return out;
    }
}
