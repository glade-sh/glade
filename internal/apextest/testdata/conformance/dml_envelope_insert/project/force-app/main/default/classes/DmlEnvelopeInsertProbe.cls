public with sharing class DmlEnvelopeInsertProbe {
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
        Map<String, String> got = new Map<String, String>();
        try { insert badAcc; got.put('stmt_req_acc','ok'); } catch (Exception e) { got.put('stmt_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(badAcc); got.put('db1_req_acc','ok'); } catch (Exception e) { got.put('db1_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(badAcc, true); got.put('db1t_req_acc','ok'); } catch (Exception e) { got.put('db1t_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(new List<Account>{badAcc}); got.put('dbl_req_acc','ok'); } catch (Exception e) { got.put('dbl_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(new List<Account>{badAcc}, true); got.put('dblt_req_acc','ok'); } catch (Exception e) { got.put('dblt_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(new List<Account>{badAcc}, true, AccessLevel.USER_MODE); got.put('dblu_req_acc','ok'); } catch (Exception e) { got.put('dblu_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(new List<Account>{badAcc}, true, AccessLevel.SYSTEM_MODE); got.put('dbls_req_acc','ok'); } catch (Exception e) { got.put('dbls_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(badAcc, true, AccessLevel.USER_MODE); got.put('db1u_req_acc','ok'); } catch (Exception e) { got.put('db1u_req_acc', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { insert badOpp; got.put('stmt_req_opp','ok'); } catch (Exception e) { got.put('stmt_req_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(badOpp); got.put('db1_req_opp','ok'); } catch (Exception e) { got.put('db1_req_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(new List<Opportunity>{badOpp}, true); got.put('dblt_req_opp','ok'); } catch (Exception e) { got.put('dblt_req_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(new List<Opportunity>{goodOpp.clone(), badOpp}); got.put('dbl_row1_opp','ok'); } catch (Exception e) { got.put('dbl_row1_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { insert new List<Opportunity>{goodOpp.clone(), badOpp}; got.put('stmt_row1_opp','ok'); } catch (Exception e) { got.put('stmt_row1_opp', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(longAcc); got.put('db1_toolong','ok'); } catch (Exception e) { got.put('db1_toolong', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { insert longAcc; got.put('stmt_toolong','ok'); } catch (Exception e) { got.put('stmt_toolong', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { Database.insert(withId); got.put('db1_withid','ok'); } catch (Exception e) { got.put('db1_withid', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        try { insert withId; got.put('stmt_withid','ok'); } catch (Exception e) { got.put('stmt_withid', e.getTypeName() + '|' + (e.getMessage()).replace(liveId18,'<live>').replace(liveId18.left(15),'<live>').replace(goneId18,'<gone>').replace(goneId18.left(15),'<gone>')); }
        return got;
    }
}
