public class LeadConvertMapProbe {
    static List<String> insertErrors = new List<String>();
    static Lead makeLead(String suffix, String first, String last, String company, String phoneTail) {
        Lead record = new Lead(
            Salutation = 'Dr.', FirstName = first, LastName = last, Title = 'Chief ' + suffix,
            Company = company, Street = '1 Main St', City = 'Oslo', State = 'NY', PostalCode = '10001', Country = 'United States',
            Phone = '555-' + phoneTail, MobilePhone = '556-' + phoneTail, Fax = '557-' + phoneTail, Email = first.toLowerCase() + '.' + last.toLowerCase() + '@' + company.replace(' ', '').toLowerCase() + '.example',
            Website = 'https://example.com/' + suffix, LeadSource = 'Web',
            Industry = 'Banking', Rating = 'Hot', AnnualRevenue = 1234, NumberOfEmployees = 12,
            Description = 'desc ' + suffix
        );
        Database.DMLOptions options = new Database.DMLOptions();
        options.duplicateRuleHeader.allowSave = true;
        Database.SaveResult saved = Database.insert(record, options);
        if (!saved.isSuccess()) { List<String> messages = new List<String>(); for (Database.Error issue : saved.getErrors()) messages.add(String.valueOf(issue.getStatusCode()) + ':' + issue.getMessage() + ':' + issue.getFields()); insertErrors.addAll(messages); }
        record.HasOptedOutOfEmail = true;
        record.DoNotCall = true;
        record.HasOptedOutOfFax = true;
        update as system record;
        return record;
    }
    static Database.LeadConvertResult convert(Lead record, Boolean skipOpportunity, String oppName) {
        Database.LeadConvert request = new Database.LeadConvert();
        request.setLeadId(record.Id);
        LeadStatus status = [SELECT MasterLabel FROM LeadStatus WHERE IsConverted = true WITH SYSTEM_MODE LIMIT 1];
        request.setConvertedStatus(status.MasterLabel);
        request.setDoNotCreateOpportunity(skipOpportunity);
        if (oppName != null) request.setOpportunityName(oppName);
        Database.DMLOptions options = new Database.DMLOptions();
        options.duplicateRuleHeader.allowSave = true;
        return Database.convertLead(request, options);
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        Lead record = makeLead('A', 'Xenia', 'Quillfeather', 'Zephyr Holdings', '0100');
        result.put('insertErrors', insertErrors);
        Database.LeadConvertResult converted = convert(record, false, 'Probe Opp');
        result.put('success', converted.isSuccess());
        if (!converted.isSuccess()) { List<String> messages = new List<String>(); for (Database.Error issue : converted.getErrors()) messages.add(String.valueOf(issue.getStatusCode()) + ':' + issue.getMessage()); result.put('errors', messages); return result; }
        result.put('hasContact', converted.getContactId() != null);
        result.put('hasAccount', converted.getAccountId() != null);
        result.put('hasOpportunity', converted.getOpportunityId() != null);
        Contact contact = [SELECT Salutation, FirstName, LastName, Title, Email, Phone, MobilePhone, Fax, Description, LeadSource,
            MailingStreet, MailingCity, MailingState, MailingPostalCode, MailingCountry, HasOptedOutOfEmail, DoNotCall, HasOptedOutOfFax, AccountId
            FROM Contact WHERE Id = :converted.getContactId() WITH SYSTEM_MODE];
        Account account = [SELECT Name, Phone, Fax, Website, Industry, AnnualRevenue, NumberOfEmployees, Description, Rating,
            BillingStreet, BillingCity, BillingState, BillingPostalCode, BillingCountry, ShippingStreet, Type
            FROM Account WHERE Id = :converted.getAccountId() WITH SYSTEM_MODE];
        Opportunity opp = [SELECT Name, LeadSource, Description, StageName, AccountId, Amount FROM Opportunity WHERE Id = :converted.getOpportunityId() WITH SYSTEM_MODE];
        Lead after = [SELECT IsConverted, Status, ConvertedAccountId, ConvertedContactId, ConvertedOpportunityId FROM Lead WHERE Id = :record.Id WITH SYSTEM_MODE];
        Map<String,Object> contactRow = new Map<String,Object>();
        for (String field : new List<String>{'Salutation','FirstName','LastName','Title','Email','Phone','MobilePhone','Fax','Description','LeadSource','MailingStreet','MailingCity','MailingState','MailingPostalCode','MailingCountry','HasOptedOutOfEmail','DoNotCall','HasOptedOutOfFax'}) contactRow.put(field, contact.get(field));
        contactRow.put('accountMatches', contact.AccountId == converted.getAccountId());
        Map<String,Object> accountRow = new Map<String,Object>();
        for (String field : new List<String>{'Name','Phone','Fax','Website','Industry','AnnualRevenue','NumberOfEmployees','Description','Rating','BillingStreet','BillingCity','BillingState','BillingPostalCode','BillingCountry','ShippingStreet','Type'}) accountRow.put(field, account.get(field));
        Map<String,Object> oppRow = new Map<String,Object>();
        for (String field : new List<String>{'Name','LeadSource','Description','StageName','Amount'}) oppRow.put(field, opp.get(field));
        oppRow.put('accountMatches', opp.AccountId == converted.getAccountId());
        result.put('contact', contactRow);
        result.put('account', accountRow);
        result.put('opportunity', oppRow);
        result.put('leadAfter', new Map<String,Object>{
            'IsConverted' => after.IsConverted, 'accountMatches' => after.ConvertedAccountId == converted.getAccountId(),
            'contactMatches' => after.ConvertedContactId == converted.getContactId(), 'oppMatches' => after.ConvertedOpportunityId == converted.getOpportunityId() });
        // Default names without an explicit opportunity name and with the opportunity skipped.
        Lead second = makeLead('B', 'Orson', 'Mapletree', 'Basalt Works', '0200');
        Database.LeadConvertResult plain = convert(second, false, null);
        Opportunity defaultOpp = [SELECT Name FROM Opportunity WHERE Id = :plain.getOpportunityId() WITH SYSTEM_MODE];
        result.put('defaultOpportunityName', defaultOpp.Name);
        Lead third = makeLead('C', 'Petra', 'Lundgren', 'Cobalt Bakery', '0300');
        Database.LeadConvertResult skipped = convert(third, true, null);
        result.put('skippedOpportunity', skipped.getOpportunityId() == null);
        // Conversion into an existing account and contact leaves their stored values alone.
        Account existing = new Account(Name = 'Existing Co', Phone = '999', Industry = 'Energy');
        Database.DMLOptions seedOptions = new Database.DMLOptions();
        seedOptions.duplicateRuleHeader.allowSave = true;
        Database.insert(existing, seedOptions);
        Contact existingContact = new Contact(AccountId = existing.Id, LastName = 'Existing', Email = 'keep@example.com', Title = 'KeepTitle');
        Database.insert(existingContact, seedOptions);
        Lead fourth = makeLead('D', 'Zeke', 'Dunnington', 'Umber Rail', '0400');
        Database.LeadConvert mergeRequest = new Database.LeadConvert();
        mergeRequest.setLeadId(fourth.Id);
        mergeRequest.setAccountId(existing.Id);
        mergeRequest.setContactId(existingContact.Id);
        mergeRequest.setDoNotCreateOpportunity(true);
        mergeRequest.setConvertedStatus([SELECT MasterLabel FROM LeadStatus WHERE IsConverted = true WITH SYSTEM_MODE LIMIT 1].MasterLabel);
        Database.DMLOptions mergeOptions = new Database.DMLOptions();
        mergeOptions.duplicateRuleHeader.allowSave = true;
        Database.LeadConvertResult mergeResult = Database.convertLead(mergeRequest, mergeOptions);
        Contact keptContact = [SELECT Email, Title, Phone, FirstName, LastName FROM Contact WHERE Id = :existingContact.Id WITH SYSTEM_MODE];
        Account keptAccount = [SELECT Name, Phone, Industry, Website FROM Account WHERE Id = :existing.Id WITH SYSTEM_MODE];
        result.put('mergeSuccess', mergeResult.isSuccess());
        result.put('mergedContact', new Map<String,Object>{ 'Email' => keptContact.Email, 'Title' => keptContact.Title, 'Phone' => keptContact.Phone, 'FirstName' => keptContact.FirstName, 'LastName' => keptContact.LastName });
        result.put('mergedAccount', new Map<String,Object>{ 'Name' => keptAccount.Name, 'Phone' => keptAccount.Phone, 'Industry' => keptAccount.Industry, 'Website' => keptAccount.Website });
        return result;
    }
}
