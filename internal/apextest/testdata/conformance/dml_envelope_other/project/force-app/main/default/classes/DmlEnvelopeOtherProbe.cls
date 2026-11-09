public with sharing class DmlEnvelopeOtherProbe {
    public static Map<String, String> observe() {
        Account live = new Account(Name = 'live');
        insert live;
        Account gone = new Account(Name = 'gone');
        insert gone;
        delete gone;
        String liveId18 = String.valueOf(live.Id);
        String goneId18 = String.valueOf(gone.Id);
        Opportunity goodOpp = new Opportunity(Name = 'g', StageName = 'Prospecting', CloseDate = Date.newInstance(2030, 1, 1));
        Opportunity badOpp = new Opportunity();
        Account badAcc = new Account();
        Account longAcc = new Account(Name = 'x'.repeat(300));
        Account withId = new Account(Id = live.Id, Name = 'y');
        Account nullName = new Account(Id = live.Id, Name = null);
        Database.DMLOptions opts = new Database.DMLOptions();
        opts.optAllOrNone = true;
        Map<String, String> got = new Map<String, String>();
        try { Database.upsert(badOpp); got.put('db1_ups_opp','ok'); } catch (Exception e) { got.put('db1_ups_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { upsert badOpp; got.put('stmt_ups_opp','ok'); } catch (Exception e) { got.put('stmt_ups_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.upsert(new List<Opportunity>{badOpp}, true); got.put('dblt_ups_opp','ok'); } catch (Exception e) { got.put('dblt_ups_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.update(new Account(Id=gone.Id, Name='z')); got.put('db1_upd_gone','ok'); } catch (Exception e) { got.put('db1_upd_gone', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { update new Account(Id=gone.Id, Name='z'); got.put('stmt_upd_gone','ok'); } catch (Exception e) { got.put('stmt_upd_gone', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.delete(gone); got.put('db1_del_gone','ok'); } catch (Exception e) { got.put('db1_del_gone', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { delete gone; got.put('stmt_del_gone','ok'); } catch (Exception e) { got.put('stmt_del_gone', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.undelete(live); got.put('db1_undel_live','ok'); } catch (Exception e) { got.put('db1_undel_live', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { undelete live; got.put('stmt_undel_live','ok'); } catch (Exception e) { got.put('stmt_undel_live', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.delete(new List<Account>{new Account(), gone}); got.put('dbl_del_noid_row0','ok'); } catch (Exception e) { got.put('dbl_del_noid_row0', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { delete new Account(); got.put('stmt_del_noid','ok'); } catch (Exception e) { got.put('stmt_del_noid', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        { Database.SaveResult[] r = Database.insert(new List<Opportunity>{badOpp}, false); got.put('ctl_false', '' + r[0].isSuccess() + '|' + r[0].getErrors()[0].getStatusCode() + '|' + r[0].getErrors()[0].getMessage() + '|' + r[0].getErrors()[0].getFields()); }
        try { insert new List<Account>{null}; got.put('ins_null_elem','ok'); } catch (Exception e) { got.put('ins_null_elem', e.getTypeName() + '|' + e.getMessage()); }
        try { insert new List<Account>{badAcc, null}; got.put('ins_null_elem1','ok'); } catch (Exception e) { got.put('ins_null_elem1', e.getTypeName() + '|' + e.getMessage()); }
        try { update new List<Account>{null}; got.put('upd_null_elem','ok'); } catch (Exception e) { got.put('upd_null_elem', e.getTypeName() + '|' + e.getMessage()); }
        try { upsert new List<Account>{null}; got.put('ups_null_elem','ok'); } catch (Exception e) { got.put('ups_null_elem', e.getTypeName() + '|' + e.getMessage()); }
        try { undelete new List<Account>{null}; got.put('undel_null_elem','ok'); } catch (Exception e) { got.put('undel_null_elem', e.getTypeName() + '|' + e.getMessage()); }
        try { Database.delete(new List<Account>{null}); got.put('db_del_null_elem','ok'); } catch (Exception e) { got.put('db_del_null_elem', e.getTypeName() + '|' + e.getMessage()); }
        try { Database.update(new List<Account>{null}, false); got.put('db_upd_null_elem','ok'); } catch (Exception e) { got.put('db_upd_null_elem', e.getTypeName() + '|' + e.getMessage()); }
        try { delete new List<Account>{live, new Account()}; got.put('del_noid_row1','ok'); } catch (Exception e) { got.put('del_noid_row1', e.getTypeName() + '|' + e.getMessage()); }
        try { undelete new Account(); got.put('undel_noid','ok'); } catch (Exception e) { got.put('undel_noid', e.getTypeName() + '|' + e.getMessage()); }
        try { undelete new List<Account>{live, new Account()}; got.put('undel_noid_row1','ok'); } catch (Exception e) { got.put('undel_noid_row1', e.getTypeName() + '|' + e.getMessage()); }
        try { Database.delete(new Account(), false); got.put('db_del_noid','ok'); } catch (Exception e) { got.put('db_del_noid', e.getTypeName() + '|' + e.getMessage()); }
        try { Database.update(nullName); got.put('upd_default_nullname','ok'); } catch (Exception e) { got.put('upd_default_nullname', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.update(nullName, AccessLevel.USER_MODE); got.put('upd_user_nullname','ok'); } catch (Exception e) { got.put('upd_user_nullname', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.update(new List<Account>{nullName}, true, AccessLevel.SYSTEM_MODE); got.put('upd_bool_sys_nullname','ok'); } catch (Exception e) { got.put('upd_bool_sys_nullname', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.update(nullName, opts); got.put('upd_opts_nullname','ok'); } catch (Exception e) { got.put('upd_opts_nullname', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(badAcc, opts); got.put('ins_opts_req','ok'); } catch (Exception e) { got.put('ins_opts_req', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.delete(gone, true, AccessLevel.USER_MODE); got.put('del_user_gone','ok'); } catch (Exception e) { got.put('del_user_gone', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.upsert(badOpp, AccessLevel.USER_MODE); got.put('ups_user_req','ok'); } catch (Exception e) { got.put('ups_user_req', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        return got;
    }
}
