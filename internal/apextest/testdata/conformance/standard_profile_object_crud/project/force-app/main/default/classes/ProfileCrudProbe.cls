public class ProfileCrudProbe {
    private static String bits(List<Boolean> b) {
        String s = '';
        for (Boolean x : b) s += x ? 'T' : 'F';
        return s;
    }
    private static String obj(Schema.SObjectType t) {
        Schema.DescribeSObjectResult d = t.getDescribe();
        return bits(new List<Boolean>{ d.isAccessible(), d.isCreateable(), d.isUpdateable(), d.isDeletable(), d.isUndeletable() });
    }
    private static String fld(Schema.DescribeFieldResult d) {
        return bits(new List<Boolean>{ d.isAccessible(), d.isCreateable(), d.isUpdateable() });
    }
    public static Map<String,Object> observeCurrentUser() {
        Map<String,Object> r = new Map<String,Object>();
        r.put('Lead', obj(Lead.SObjectType));
        r.put('Campaign', obj(Campaign.SObjectType));
        r.put('Account', obj(Account.SObjectType));
        r.put('Contact', obj(Contact.SObjectType));
        r.put('Opportunity', obj(Opportunity.SObjectType));
        r.put('Case', obj(Case.SObjectType));
        r.put('Product2', obj(Product2.SObjectType));
        r.put('Contract', obj(Contract.SObjectType));
        r.put('Order', obj(Order.SObjectType));
        r.put('Task', obj(Task.SObjectType));
        r.put('Event', obj(Event.SObjectType));
        r.put('AuthProvider', obj(AuthProvider.SObjectType));
        r.put('Lead.LastName', fld(Lead.LastName.getDescribe()));
        r.put('Lead.Company', fld(Lead.Company.getDescribe()));
        r.put('Campaign.Name', fld(Campaign.Name.getDescribe()));
        r.put('Account.Name', fld(Account.Name.getDescribe()));
        r.put('Account.Phone', fld(Account.Phone.getDescribe()));
        r.put('Opportunity.Name', fld(Opportunity.Name.getDescribe()));
        r.put('Case.Subject', fld(Case.Subject.getDescribe()));
        r.put('Lead.DoNotCall', fld(Lead.DoNotCall.getDescribe()));
        r.put('Lead.HasOptedOutOfEmail', fld(Lead.HasOptedOutOfEmail.getDescribe()));
        r.put('Contact.DoNotCall', fld(Contact.DoNotCall.getDescribe()));
        return r;
    }
    public static Map<String,Object> observe() {
        return new Map<String,Object>{ 'admin' => observeCurrentUser() };
    }
}
