@isTest private class PclEditTest {
    static String fire(Id uid, Id rid) {
        UserRecordAccess a = [SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId = :uid AND RecordId = :rid];
        return (a.HasReadAccess ? '1' : '0') + (a.HasEditAccess ? '1' : '0');
    }
    @isTest static void run() {
        UserRole r = new UserRole(Name = 'Pcl Owner Role'); insert r;
        User owner = PclSupport.newUser('pclown', PclSupport.profileId('System Administrator'), r.Id, null);
        insert owner;
        Case c; Lead l;
        System.runAs(owner) { c = new Case(Subject = 'PclOther'); insert c; l = new Lead(LastName = 'PclLead', Company = 'PclCo'); insert l; }
        User u = PclSupport.newUser('pclstd', PclSupport.profileId('Standard User'), null, null);
        insert u;
        String m = 'uraCase=' + fire(u.Id, c.Id) + '|uraLead=' + fire(u.Id, l.Id);
        System.runAs(u) {
            try { c.Subject = 'X'; update c; m += '|updCase=ok'; } catch (Exception e) { m += '|updCase=' + e.getTypeName(); }
            try { l.Company = 'X'; update l; m += '|updLead=ok'; } catch (Exception e) { m += '|updLead=' + e.getTypeName() + ':' + e.getMessage().left(60); }
        }
        System.assert(false, m);
    }
}
