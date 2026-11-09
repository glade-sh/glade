@isTest
private class C4PortalTest {
    static User owner;
    static List<Account> accounts;
    static Contact contact;
    static void setup() {
        UserRole r = new UserRole(Name = 'C4 Owner Role');
        insert r;
        owner = C4PortalProbe.newUser('c4own', C4PortalProbe.profileId('System Administrator'), r.Id, null);
        insert owner;
        System.runAs(owner) {
            Account parent = new Account(Name = 'C4Parent');
            insert parent;
            accounts = new List<Account>{parent, new Account(Name = 'C4Child1', ParentId = parent.Id), new Account(Name = 'C4Child2', ParentId = parent.Id)};
            insert new List<Account>{accounts[1], accounts[2]};
            contact = new Contact(LastName = 'C4Contact', AccountId = parent.Id);
            insert contact;
        }
    }
    static User portal(String profileName) {
        User u = C4PortalProbe.newUser('c4prt', C4PortalProbe.profileId(profileName), null, contact.Id);
        insert u;
        return u;
    }
    static void observe(User u) {
        Map<String, Object> o = C4PortalProbe.shares(accounts, u, owner);
        System.runAs(u) {
            o.put('visibleAccounts', C4PortalProbe.visibleAccounts());
            o.put('visibleContacts', C4PortalProbe.visibleContacts());
        }
        System.assert(false, 'OBS|' + JSON.serialize(o));
    }
    @isTest static void cc_immediate() {
        setup();
        User u = portal('Customer Community User');
        observe(u);
    }
    @isTest static void cc_stoptest() {
        setup();
        User u = portal('Customer Community User');
        Test.startTest();
        Test.stopTest();
        observe(u);
    }
    @isTest static void cc_inside() {
        setup();
        Test.startTest();
        User u = portal('Customer Community User');
        Test.stopTest();
        observe(u);
    }
    @isTest static void ccp_immediate() {
        setup();
        User u = portal('Customer Community Plus User');
        observe(u);
    }
    @isTest static void ccp_stoptest() {
        setup();
        User u = portal('Customer Community Plus User');
        Test.startTest();
        Test.stopTest();
        observe(u);
    }
    @isTest static void ccp_inside() {
        setup();
        Test.startTest();
        User u = portal('Customer Community Plus User');
        Test.stopTest();
        observe(u);
    }
    @isTest static void ccl_immediate() {
        setup();
        User u = portal('Customer Community Login User');
        observe(u);
    }
    @isTest static void ccl_stoptest() {
        setup();
        User u = portal('Customer Community Login User');
        Test.startTest();
        Test.stopTest();
        observe(u);
    }
    @isTest static void ccl_inside() {
        setup();
        Test.startTest();
        User u = portal('Customer Community Login User');
        Test.stopTest();
        observe(u);
    }
}
