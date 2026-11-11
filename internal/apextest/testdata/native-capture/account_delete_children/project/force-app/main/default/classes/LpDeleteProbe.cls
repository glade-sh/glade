public class LpDeleteProbe {
    static Map<String, String> masks = new Map<String, String>();
    static List<String> out;

    static String mask(String text) {
        if (text == null) return null;
        for (String key : masks.keySet()) text = text.replace(key, masks.get(key));
        return text;
    }
    static void row(String id, String observation) { out.add(id + '|' + mask(observation)); }
    static String state(String objectName, Id recordId) {
        List<SObject> rows = Database.query('SELECT Id, IsDeleted FROM ' + objectName + ' WHERE Id = :recordId ALL ROWS');
        if (rows.isEmpty()) return 'gone';
        return ((Boolean) rows[0].get('IsDeleted')) ? 'deleted' : 'live';
    }
    static Account account(String name) {
        Account a = new Account(Name = 'LP ' + name);
        insert a;
        masks.put(a.Id, '<account>');
        masks.put(String.valueOf(a.Id).substring(0, 15), '<account15>');
        return a;
    }
    static Contract contract(Id accountId, String status, Date startDate) {
        Contract c = new Contract(AccountId = accountId, StartDate = startDate, ContractTerm = 12, Status = 'Draft');
        insert c;
        if (status != 'Draft') {
            c.Status = status;
            update c;
        }
        c = [SELECT Id, ContractNumber, Status, StatusCode FROM Contract WHERE Id = :c.Id];
        masks.put(c.Id, '<contract>');
        masks.put(String.valueOf(c.Id).substring(0, 15), '<contract15>');
        masks.put(c.ContractNumber, '<contractNumber>');
        return c;
    }
    static String deleteResult(Account a, Boolean allOrNone) {
        if (allOrNone) {
            try {
                delete a;
                return 'ok';
            } catch (DmlException e) {
                return 'DmlException ' + e.getDmlType(0) + ' ' + e.getDmlFieldNames(0) + ' ' + e.getDmlMessage(0);
            }
        }
        Database.DeleteResult r = Database.delete(a, false);
        if (r.isSuccess()) return 'ok';
        Database.Error error = r.getErrors()[0];
        return 'result ' + error.getStatusCode() + ' ' + error.getFields() + ' ' + error.getMessage();
    }

    public static List<String> run() {
        out = new List<String>();
        Savepoint sp;
        // D01 Draft Contract
        sp = Database.setSavepoint();
        try {
            Account a = account('D01');
            Contract c = contract(a.Id, 'Draft', Date.today());
            row('D01 contract status', c.Status + '/' + c.StatusCode);
            row('D01 delete', deleteResult(a, true));
            row('D01 contract', state('Contract', c.Id));
            row('D01 account', state('Account', a.Id));
            try {
                undelete a;
                row('D01 undelete', 'ok');
            } catch (DmlException e) {
                row('D01 undelete', 'DmlException ' + e.getDmlType(0) + ' ' + e.getDmlMessage(0));
            }
            row('D01 contract after undelete', state('Contract', c.Id));
        } catch (Exception e) { row('D01 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D02 Activated Contract
        sp = Database.setSavepoint();
        try {
            Account a = account('D02');
            Contract c = contract(a.Id, 'Activated', Date.today());
            row('D02 contract status', c.Status + '/' + c.StatusCode);
            row('D02 delete', deleteResult(a, true));
            row('D02 contract', state('Contract', c.Id));
            row('D02 account', state('Account', a.Id));
            row('D02 partial', deleteResult(a, false));
        } catch (Exception e) { row('D02 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D03 Activated Contract whose end date has passed
        sp = Database.setSavepoint();
        try {
            Account a = account('D03');
            Contract c = contract(a.Id, 'Activated', Date.today().addMonths(-14));
            row('D03 contract status', c.Status + '/' + c.StatusCode);
            row('D03 delete', deleteResult(a, true));
            row('D03 contract', state('Contract', c.Id));
        } catch (Exception e) { row('D03 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D04 Expired status set directly
        sp = Database.setSavepoint();
        try {
            Account a = account('D04');
            Contract c = contract(a.Id, 'Draft', Date.today().addMonths(-14));
            try {
                Contract u = new Contract(Id = c.Id, Status = 'Expired');
                update u;
                row('D04 set Expired', 'ok');
            } catch (DmlException e) {
                row('D04 set Expired', 'DmlException ' + e.getDmlType(0) + ' ' + e.getDmlFieldNames(0) + ' ' + e.getDmlMessage(0));
            }
            c = [SELECT Status, StatusCode FROM Contract WHERE Id = :c.Id];
            row('D04 contract status', c.Status + '/' + c.StatusCode);
            row('D04 delete', deleteResult(a, true));
            row('D04 contract', state('Contract', c.Id));
        } catch (Exception e) { row('D04 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D05 In Approval Process Contract
        sp = Database.setSavepoint();
        try {
            Account a = account('D05');
            Contract c = contract(a.Id, 'In Approval Process', Date.today());
            row('D05 contract status', c.Status + '/' + c.StatusCode);
            row('D05 delete', deleteResult(a, true));
            row('D05 contract', state('Contract', c.Id));
        } catch (Exception e) { row('D05 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D06 Draft and Activated Contracts together
        sp = Database.setSavepoint();
        try {
            Account a = account('D06');
            Contract d = contract(a.Id, 'Draft', Date.today());
            masks.put(d.Id, '<draft>');
            masks.put(String.valueOf(d.Id).substring(0, 15), '<draft15>');
            Contract c = contract(a.Id, 'Activated', Date.today());
            row('D06 delete', deleteResult(a, true));
            row('D06 draft', state('Contract', d.Id));
            row('D06 activated', state('Contract', c.Id));
        } catch (Exception e) { row('D06 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D07 open Opportunity
        sp = Database.setSavepoint();
        try {
            Account a = account('D07');
            Opportunity o = new Opportunity(AccountId = a.Id, Name = 'LP O', StageName = 'Prospecting', CloseDate = Date.today());
            insert o;
            row('D07 delete', deleteResult(a, true));
            row('D07 opportunity', state('Opportunity', o.Id));
        } catch (Exception e) { row('D07 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D08 Closed Won Opportunity
        sp = Database.setSavepoint();
        try {
            Account a = account('D08');
            Opportunity o = new Opportunity(AccountId = a.Id, Name = 'LP W', StageName = 'Closed Won', CloseDate = Date.today());
            insert o;
            row('D08 delete', deleteResult(a, true));
            row('D08 opportunity', state('Opportunity', o.Id));
        } catch (Exception e) { row('D08 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D09 Case on the Account
        sp = Database.setSavepoint();
        try {
            Account a = account('D09');
            Case k = new Case(AccountId = a.Id, Subject = 'LP K', Status = 'New', Origin = 'Web');
            insert k;
            k = [SELECT Id, CaseNumber FROM Case WHERE Id = :k.Id];
            masks.put(k.Id, '<case>');
            masks.put(String.valueOf(k.Id).substring(0, 15), '<case15>');
            masks.put(k.CaseNumber, '<caseNumber>');
            row('D09 delete', deleteResult(a, true));
            row('D09 case', state('Case', k.Id));
            row('D09 account', state('Account', a.Id));
            row('D09 partial', deleteResult(a, false));
        } catch (Exception e) { row('D09 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D10 closed Case on the Account
        sp = Database.setSavepoint();
        try {
            Account a = account('D10');
            Case k = new Case(AccountId = a.Id, Subject = 'LP C', Status = 'Closed', Origin = 'Web');
            insert k;
            row('D10 delete', deleteResult(a, true));
            row('D10 case', state('Case', k.Id));
        } catch (Exception e) { row('D10 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D11 Contact
        sp = Database.setSavepoint();
        try {
            Account a = account('D11');
            Contact p = new Contact(AccountId = a.Id, LastName = 'LP P');
            insert p;
            row('D11 delete', deleteResult(a, true));
            row('D11 contact', state('Contact', p.Id));
        } catch (Exception e) { row('D11 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D12 Contact with a Case on the Contact only
        sp = Database.setSavepoint();
        try {
            Account a = account('D12');
            Contact p = new Contact(AccountId = a.Id, LastName = 'LP Q');
            insert p;
            Case k = new Case(ContactId = p.Id, Subject = 'LP Q', Status = 'New', Origin = 'Web');
            insert k;
            k = [SELECT Id, AccountId, CaseNumber FROM Case WHERE Id = :k.Id];
            masks.put(k.Id, '<case>');
            masks.put(String.valueOf(k.Id).substring(0, 15), '<case15>');
            masks.put(k.CaseNumber, '<caseNumber>');
            row('D12 case account', k.AccountId == a.Id ? 'account' : String.valueOf(k.AccountId));
            row('D12 delete', deleteResult(a, true));
            row('D12 contact', state('Contact', p.Id));
            row('D12 case', state('Case', k.Id));
        } catch (Exception e) { row('D12 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D13 Draft Order
        sp = Database.setSavepoint();
        try {
            Account a = account('D13');
            Order r = new Order(AccountId = a.Id, EffectiveDate = Date.today(), Status = 'Draft');
            insert r;
            row('D13 delete', deleteResult(a, true));
            row('D13 order', state('Order', r.Id));
        } catch (Exception e) { row('D13 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D14 Draft Contract with a Draft Order on it
        sp = Database.setSavepoint();
        try {
            Account a = account('D14');
            Contract c = contract(a.Id, 'Draft', Date.today());
            Order r = new Order(AccountId = a.Id, ContractId = c.Id, EffectiveDate = Date.today(), Status = 'Draft');
            insert r;
            row('D14 delete', deleteResult(a, true));
            row('D14 contract', state('Contract', c.Id));
            row('D14 order', state('Order', r.Id));
        } catch (Exception e) { row('D14 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D15 Activated Contract with Opportunity and Contact: all or none
        sp = Database.setSavepoint();
        try {
            Account a = account('D15');
            Contract c = contract(a.Id, 'Activated', Date.today());
            Opportunity o = new Opportunity(AccountId = a.Id, Name = 'LP O', StageName = 'Prospecting', CloseDate = Date.today());
            insert o;
            Contact p = new Contact(AccountId = a.Id, LastName = 'LP P');
            insert p;
            row('D15 delete', deleteResult(a, true));
            row('D15 opportunity', state('Opportunity', o.Id));
            row('D15 contact', state('Contact', p.Id));
            row('D15 contract', state('Contract', c.Id));
        } catch (Exception e) { row('D15 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D16 Opportunity, Contact and Draft Contract together
        sp = Database.setSavepoint();
        try {
            Account a = account('D16');
            Contract c = contract(a.Id, 'Draft', Date.today());
            Opportunity o = new Opportunity(AccountId = a.Id, Name = 'LP O', StageName = 'Prospecting', CloseDate = Date.today());
            insert o;
            Contact p = new Contact(AccountId = a.Id, LastName = 'LP P');
            insert p;
            row('D16 delete', deleteResult(a, true));
            row('D16 opportunity', state('Opportunity', o.Id));
            row('D16 contact', state('Contact', p.Id));
            row('D16 contract', state('Contract', c.Id));
        } catch (Exception e) { row('D16 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        // D17 delete the Contract directly: Draft, then Activated
        sp = Database.setSavepoint();
        try {
            Account a = account('D17');
            Contract d = contract(a.Id, 'Draft', Date.today());
            try { delete d; row('D17 delete draft contract', 'ok'); } catch (DmlException e) { row('D17 delete draft contract', 'DmlException ' + e.getDmlType(0) + ' ' + e.getDmlFieldNames(0) + ' ' + e.getDmlMessage(0)); }
            Contract c = contract(a.Id, 'Activated', Date.today());
            try { delete c; row('D17 delete activated contract', 'ok'); } catch (DmlException e) { row('D17 delete activated contract', 'DmlException ' + e.getDmlType(0) + ' ' + e.getDmlFieldNames(0) + ' ' + e.getDmlMessage(0)); }
        } catch (Exception e) { row('D17 error', e.getTypeName() + ' ' + e.getMessage()); }
        Database.rollback(sp);
        return out;
    }
}
