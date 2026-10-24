public class LindenProbe {
    public static void observe(Integer part) { System.assert(false, 'LindenROWS\n' + run(part)); }
    public static String run(Integer part) {
        List<String> out = new List<String>();
        if (part == 1) {
        Account a = new Account(Name='Target'); insert a;
        Account b = new Account(Name='Other'); insert b;
        // P1: prior account set, prior title T1
        Contact c1 = new Contact(FirstName='a', LastName='Target', AccountId=b.Id, Title='T1'); insert c1;
        // P2: prior account null, prior title null
        Contact c2 = new Contact(FirstName='a', LastName='Nomatch'); insert c2;
        // P3: prior account null, title other
        Contact c3 = new Contact(FirstName='a', LastName='Nomatch3', Title='T2'); insert c3;
        c1.FirstName='b'; c2.FirstName='b'; c3.FirstName='b';
        update c1; update c2; update c3;
        for (Contact c : [SELECT LastName, AccountId, Department, AssistantName, Description FROM Contact ORDER BY LastName]) {
            out.add('CONTACT|' + c.LastName + '|acct=' + (c.AccountId == null ? 'null' : (c.AccountId == a.Id ? 'target' : 'other')) + '|dept=' + c.Department + '|asst=' + c.AssistantName + '|desc=' + c.Description);
        }
        Contact u1 = new Contact(LastName='CU1', Title='T1', LeadSource='Web'); insert u1;
        out.add(dump('CU1-insert', u1.Id));
        u1.LastName='CU1b'; update u1;
        out.add(dump('CU1-update', u1.Id));
        Contact u2 = new Contact(LastName='CU2'); insert u2;
        out.add(dump('CU2-insert', u2.Id));
        u2.LastName='CU2b'; update u2;
        out.add(dump('CU2-update', u2.Id));
        Contact b1 = new Contact(LastName='BS1', Title='T1', Department='D1'); insert b1;
        out.add(dumpB('BS1-insert', b1.Id));
        b1.Department='D2'; update b1;
        out.add(dumpB('BS1-update', b1.Id));
        Contact b2 = new Contact(LastName='BS2', Department='D3'); insert b2;
        out.add(dumpB('BS2-insert', b2.Id));
        b2.Department='D4'; update b2;
        out.add(dumpB('BS2-update', b2.Id));
        Contact b3 = new Contact(LastName='BS3', Title='T2', Department='D1'); insert b3;
        b3.Department='D2'; update b3;
        out.add(dumpB('BS3-update', b3.Id));
        }
        if (part == 2) {
        Contact v1 = new Contact(LastName='V1', Title='x', Birthdate=Date.newInstance(2000,1,2), Department='a'); insert v1;
        v1.Department='b'; update v1;
        out.add(dumpV('V1-set', v1.Id));
        Contact v2 = new Contact(LastName='V2', Department='a'); insert v2;
        v2.Department='b'; update v2;
        out.add(dumpV('V2-null', v2.Id));
        Account oa1 = new Account(Name='OA1', Industry='Banking', Phone='555'); insert oa1;
        Account oa2 = new Account(Name='OA2', Industry='Energy'); insert oa2;
        Contact o1 = new Contact(LastName='O1', AccountId=oa1.Id); insert o1;
        Contact o2 = new Contact(LastName='O2', AccountId=oa2.Id); insert o2;
        Contact o3 = new Contact(LastName='O3'); insert o3;
        o1.Linden_N__c='x'; o2.Linden_N__c='x'; o3.Linden_N__c='x';
        update o1; update o2; update o3;
        out.add(dumpO('O1-bank', o1.Id)); out.add(dumpO('O2-energy', o2.Id)); out.add(dumpO('O3-noacct', o3.Id));
        }
        if (part == 3) {
        Account pa1 = new Account(Name='Open', Linden_MS__c='A;B'); insert pa1;
        Account pa2 = new Account(Name='Closed', Linden_MS__c='B'); insert pa2;
        Account pa3 = new Account(Name='Open3'); insert pa3;
        Contact p1 = new Contact(LastName='P1', Department='Y', AccountId=pa1.Id, Linden_Acct__c=pa1.Id);
        Contact p2 = new Contact(LastName='P2', Department='Y', AccountId=pa2.Id, Linden_Acct__c=pa2.Id);
        Contact p3 = new Contact(LastName='P3', Department='X', AccountId=pa3.Id, Linden_Acct__c=pa3.Id);
        Contact p4 = new Contact(LastName='P4', Department='Y');
        Contact p5 = new Contact(LastName='P5', Department='X', AccountId=pa1.Id);
        insert new List<Contact>{p1,p2,p3,p4,p5};
        List<Contact> pl = new List<Contact>{p1,p2,p3,p4,p5};
        for (Contact pc : pl) pc.Linden_O__c = 'chg';
        update pl;
        for (Contact pc : [SELECT LastName, Linden_Q__c, Linden_S__c FROM Contact WHERE LastName LIKE 'P%' ORDER BY LastName]) out.add('INC|' + pc.LastName + '|q=' + pc.Linden_Q__c + '|s=' + pc.Linden_S__c);
        for (Account pa : [SELECT Name, Linden_Tx__c, Linden_Dt__c, Linden_Dd__c, LastModifiedDate FROM Account WHERE Name IN ('Open','Closed','Open3') ORDER BY Name]) out.add('PARENT|' + pa.Name + '|tx=' + pa.Linden_Tx__c + '|dt=' + (pa.Linden_Dt__c == null ? 'null' : 'set') + '|dd=' + (pa.Linden_Dd__c == null ? 'null' : (pa.Linden_Dd__c == pa.LastModifiedDate.dateGmt() ? 'gmt' : (pa.Linden_Dd__c == pa.LastModifiedDate.date() ? 'local' : 'other'))));
        Contact k1c = new Contact(LastName='KC1', Linden_U__c='prefill'); insert k1c;
        k1c.Linden_V__c='go'; update k1c;
        Contact k1r = [SELECT Linden_T__c, Linden_U__c FROM Contact WHERE Id = :k1c.Id];
        out.add('CONST|KC1|t=' + k1r.Linden_T__c + '|u=' + (k1r.Linden_U__c == null ? 'null' : ('[' + k1r.Linden_U__c + ']')));
        Contact ow = new Contact(LastName='OW1'); insert ow;
        ow.Linden_X__c='go'; update ow;
        Contact owr = [SELECT Linden_W__c, OwnerId FROM Contact WHERE Id = :ow.Id];
        out.add('OWNER|OW1|w=' + owr.Linden_W__c);
        Id rt = [SELECT Id FROM RecordType WHERE SobjectType='Account' AND DeveloperName='Linden_Closed'].Id;
        Account k1 = new Account(Name='K1', Site='Linden_Closed'); insert k1;
        Account k2 = new Account(Name='K2', Site='zzz'); insert k2;
        k1.Name='K1b'; k2.Name='K2b';
        update k1; update k2;
        out.add('TASKS|' + [SELECT COUNT() FROM Task WHERE Subject = 'linden task']);
        for (Account k : [SELECT Name, RecordTypeId, Description FROM Account WHERE Name LIKE 'K%' ORDER BY Name]) {
            out.add('ACCT|' + k.Name + '|rt=' + (k.RecordTypeId == null ? 'null' : (k.RecordTypeId == rt ? 'closed' : 'other')) + '|desc=' + k.Description);
        }
        }
        return String.join(out, '\n');
    }

    static String dump(String l, Id i) {
        Contact c = [SELECT HomePhone, MobilePhone, Fax, AssistantPhone, OtherPhone FROM Contact WHERE Id = :i];
        return 'CU|' + l + '|home=' + c.HomePhone + '|mobile=' + c.MobilePhone + '|fax=' + c.Fax + '|asst=' + c.AssistantPhone + '|other=' + c.OtherPhone;
    }

    static String dumpB(String l, Id i) {
        Contact c = [SELECT Description, AssistantName, Fax FROM Contact WHERE Id = :i];
        return 'BS|' + l + '|desc=' + c.Description + '|asst=' + c.AssistantName + '|fax=' + c.Fax;
    }

    static String dumpV(String l, Id i) {
        Contact c = [SELECT Linden_A__c, Linden_B__c, Linden_C__c, Linden_D__c, Linden_E__c, Linden_F__c, Linden_G__c, Linden_H__c, Linden_I__c FROM Contact WHERE Id = :i];
        return 'VARS|' + l + '|a=' + c.Linden_A__c + '|b=' + c.Linden_B__c + '|c=' + c.Linden_C__c + '|d=' + c.Linden_D__c + '|e=' + c.Linden_E__c + '|f=' + c.Linden_F__c + '|g=' + c.Linden_G__c + '|h=' + c.Linden_H__c + '|i=' + c.Linden_I__c;
    }

    static String dumpO(String l, Id i) {
        Contact c = [SELECT Linden_J__c, Linden_K__c, Linden_L__c, Linden_M__c FROM Contact WHERE Id = :i];
        return 'OUT|' + l + '|j=' + c.Linden_J__c + '|k=' + c.Linden_K__c + '|l=' + c.Linden_L__c + '|m=' + c.Linden_M__c;
    }
}
