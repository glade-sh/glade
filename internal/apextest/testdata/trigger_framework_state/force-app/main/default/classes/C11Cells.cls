@IsTest
private class C11Cells {
    static String msg(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static void report(String out) { System.assertEquals('', out); }

    @IsTest static void visibility() {
        String out = 'cfgCount=' + [SELECT COUNT() FROM C11Cfg__c];
        out += ';getAll=' + C11Cfg__c.getAll().size();
        out += ';seedInst=' + (C11Cfg__c.getInstance('seed') != null);
        out += ';hierFlag=' + C11Hier__c.getOrgDefaults().Flag__c;
        out += ';recCount=' + [SELECT COUNT() FROM C11Rec__c];
        report(out);
    }
    @IsTest static void noMapping() {
        String out = '';
        try { insert new C11Rec__c(Name = 'a', Kind__c = 'nope'); out = 'ok'; } catch (Exception e) { out = msg(e); }
        report(out);
    }
    @IsTest static void validationReject() {
        insert new C11Cfg__c(Name = 'bad', RtName__c = 'Alpha');
        String out = '';
        try { insert new C11Rec__c(Name = 'a', Kind__c = 'bad'); out = 'ok'; } catch (Exception e) { out = msg(e); }
        report(out);
    }
    @IsTest static void nestedReject() {
        insert new C11Cfg__c(Name = 'nested', RtName__c = 'Alpha');
        String out = '';
        try { insert new C11Rec__c(Name = 'a', Kind__c = 'nested'); out = 'ok'; } catch (Exception e) { out = msg(e); }
        report(out);
    }
    @IsTest static void rtMapped() {
        insert new C11Cfg__c(Name = 'alpha', RtName__c = 'Alpha');
        C11Rec__c r = new C11Rec__c(Name = 'a', Kind__c = 'alpha');
        insert r;
        r = [SELECT RecordType.DeveloperName FROM C11Rec__c WHERE Id = :r.Id];
        report(r.RecordType.DeveloperName);
    }
    @IsTest static void rtMissing() {
        insert new C11Cfg__c(Name = 'zed', RtName__c = 'Zzz');
        String out = '';
        try { insert new C11Rec__c(Name = 'a', Kind__c = 'zed'); out = 'ok'; } catch (Exception e) { out = msg(e); }
        report(out);
    }
    @IsTest static void bulkMixed() {
        insert new C11Cfg__c(Name = 'alpha', RtName__c = 'Alpha');
        List<Database.SaveResult> rs = Database.insert(new List<C11Rec__c>{
            new C11Rec__c(Name = 'a', Kind__c = 'alpha'), new C11Rec__c(Name = 'b', Kind__c = 'alpha')}, false);
        String out = '';
        for (Database.SaveResult s : rs) { out += s.isSuccess() + ','; }
        report(out);
    }
    static String full(Exception e) {
        String c = e.getCause() == null ? 'nocause' : e.getCause().getTypeName() + '/' + e.getCause().getMessage();
        return msg(e) + '\nSTACK=' + e.getStackTraceString() + '\nCAUSE=' + c;
    }
    @IsTest static void stackNoMapping() {
        String out = '';
        try { insert new C11Rec__c(Name = 'a', Kind__c = 'nope'); out = 'ok'; } catch (Exception e) { out = full(e); }
        report(out);
    }
    @IsTest static void stackNested() {
        insert new C11Cfg__c(Name = 'nested', RtName__c = 'Alpha');
        String out = '';
        try { insert new C11Rec__c(Name = 'a', Kind__c = 'nested'); out = 'ok'; } catch (Exception e) { out = full(e); }
        report(out);
    }
    @IsTest static void stackValidation() {
        insert new C11Cfg__c(Name = 'bad', RtName__c = 'Alpha');
        String out = '';
        try { insert new C11Rec__c(Name = 'a', Kind__c = 'bad'); out = 'ok'; } catch (Exception e) { out = full(e); }
        report(out);
    }
    static String defs() {
        C11Def__c a = new C11Def__c();
        C11Def__c b = new C11Def__c(Txt__c = 'x');
        C11Def__c c = (C11Def__c) C11Def__c.SObjectType.newSObject(null, true);
        C11Def__c d = (C11Def__c) C11Def__c.SObjectType.newSObject();
        C11Hier__c h = new C11Hier__c();
        C11Hier__c ho = C11Hier__c.getOrgDefaults();
        C11Cfg__c cf = new C11Cfg__c(Name = 'q');
        C11Def__c e = new C11Def__c(Name = 'e');
        insert e;
        e = [SELECT Flag__c, Txt__c, Num__c FROM C11Def__c WHERE Id = :e.Id];
        return 'new=' + a.Flag__c + '/' + a.Txt__c + '/' + a.Num__c
            + ';newPartial=' + b.Flag__c + '/' + b.Txt__c + '/' + b.Num__c
            + ';loadDefaults=' + c.Flag__c + '/' + c.Txt__c + '/' + c.Num__c
            + ';newSObject=' + d.Flag__c + '/' + d.Txt__c + '/' + d.Num__c
            + ';hierNew=' + h.Enf__c + ';hierOrg=' + ho.Enf__c + ';cfgNew=' + cf.Enf__c
            + ';afterInsert=' + e.Flag__c + '/' + e.Txt__c + '/' + e.Num__c;
    }
    @IsTest static void defaultsNew() { report(defs()); }
    static String sd(Boolean withRow) {
        String out = '';
        C11Hier__c h = new C11Hier__c();
        C11Hier__c ho = C11Hier__c.getOrgDefaults();
        C11Cfg__c cf = new C11Cfg__c(Name = 'q2');
        out += 'hierNew=' + h.Enf__c + '/' + h.Txt2__c + '/' + h.Num2__c;
        out += ';hierOrg=' + ho.Enf__c + '/' + ho.Txt2__c + '/' + ho.Num2__c;
        out += ';cfgNew=' + cf.Enf__c + '/' + cf.Txt2__c + '/' + cf.Num2__c;
        if (withRow) {
            insert new C11Hier__c(SetupOwnerId = UserInfo.getUserId());
            C11Hier__c hi = C11Hier__c.getInstance();
            C11Hier__c hv = C11Hier__c.getValues(UserInfo.getUserId());
            out += ';afterInsertInst=' + hi.Enf__c + '/' + hi.Txt2__c;
            out += ';afterInsertVals=' + hv.Enf__c + '/' + hv.Txt2__c;
        }
        return out;
    }
    @IsTest static void settingDefaults() { report(sd(true)); }
}
