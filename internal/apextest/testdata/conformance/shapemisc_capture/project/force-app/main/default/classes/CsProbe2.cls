public class CsProbe2 {
    static String ex(Exception e) { return e.getTypeName() + '|' + e.getMessage() + '|' + e.getLineNumber(); }
    public static List<String> run() {
        List<String> out = new List<String>();
        CsHolder h = new CsHolder();
        h.nested = new CsHolder();
        CsSub sub = new CsSub();
        try { out.add('c01_thisHierText=' + h.readThisHier()); } catch (Exception e) { out.add('c01_thisHierText=EX ' + ex(e)); }
        try { out.add('c02_chainHierId=' + h.hier.Id); } catch (Exception e) { out.add('c02_chainHierId=EX ' + ex(e)); }
        try { out.add('c03_thisListId=' + h.readThisId()); } catch (Exception e) { out.add('c03_thisListId=EX ' + ex(e)); }
        try { out.add('c04_thisHierId=' + h.readThisHierId()); } catch (Exception e) { out.add('c04_thisHierId=EX ' + ex(e)); }
        try { out.add('c05_thisListName=' + h.readThisName()); } catch (Exception e) { out.add('c05_thisListName=EX ' + ex(e)); }
        try { out.add('c06_fourSegment=' + h.nested.lst.Text__c); } catch (Exception e) { out.add('c06_fourSegment=EX ' + ex(e)); }
        try { out.add('c07_getterCallRead=' + h.getLst().Text__c); } catch (Exception e) { out.add('c07_getterCallRead=EX ' + ex(e)); }
        try { out.add('c08_selfAlias=' + h.readSelf()); } catch (Exception e) { out.add('c08_selfAlias=EX ' + ex(e)); }
        try { out.add('c09_paramChain=' + CsHolder.readParam(h)); } catch (Exception e) { out.add('c09_paramChain=EX ' + ex(e)); }
        try { out.add('c10_chainHierName=' + h.hier.Name); } catch (Exception e) { out.add('c10_chainHierName=EX ' + ex(e)); }
        try { out.add('c11_subChain=' + sub.lst.Text__c); } catch (Exception e) { out.add('c11_subChain=EX ' + ex(e)); }
        try { out.add('c12_thisAssignedNull=' + new AssignedHolder().read()); } catch (Exception e) { out.add('c12_thisAssignedNull=EX ' + ex(e)); }
        try { out.add('c13_chainListSetupOwner=' + h.hier.SetupOwnerId); } catch (Exception e) { out.add('c13_chainHierSetupOwnerId=EX ' + ex(e)); }
        try { out.add('c14_chainListOwnerlessField=' + h.lst.CreatedDate); } catch (Exception e) { out.add('c14_chainListCreatedDate=EX ' + ex(e)); }
        try { out.add('c15_chainListIsDeleted=' + h.lst.IsDeleted); } catch (Exception e) { out.add('c15_chainListIsDeleted=EX ' + ex(e)); }
        try { Account a = new Account(); Contact c = new Contact(); out.add('c16_regularSObjectMember=' + a.Parent.Name); } catch (Exception e) { out.add('c16_regularSObjectMember=EX ' + ex(e)); }
        try { out.add('c17_accountMemberChain=' + new AcctHolder().acct.Name); } catch (Exception e) { out.add('c17_accountMemberChain=EX ' + ex(e)); }
        try { out.add('c18_accountMemberThis=' + new AcctHolder().readThis()); } catch (Exception e) { out.add('c18_accountMemberThis=EX ' + ex(e)); }
        try { out.add('c19_accountMemberImplicit=' + new AcctHolder().readImplicit()); } catch (Exception e) { out.add('c19_accountMemberImplicit=EX ' + ex(e)); }
        return out;
    }
}
