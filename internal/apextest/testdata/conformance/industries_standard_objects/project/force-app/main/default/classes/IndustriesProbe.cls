public class IndustriesProbe {
    static String attempt(Id accountId, String shape) {
        try {
            if (shape == 'application_forms_empty') {
                return String.valueOf([SELECT Id, (SELECT Id FROM ApplicationForms) FROM Account WHERE Id = :accountId][0].ApplicationForms.size());
            }
            if (shape == 'business_profiles') {
                insert new BusinessProfile(AccountId = accountId);
                return String.valueOf([SELECT Id, (SELECT Id FROM BusinessProfiles) FROM Account WHERE Id = :accountId][0].BusinessProfiles.size());
            }
            if (shape == 'public_complaints') {
                insert new PublicComplaint(AccountId = accountId, IncidentDate = Date.newInstance(2026, 1, 2));
                return String.valueOf([SELECT Id, (SELECT Id FROM PublicComplaintAccount) FROM Account WHERE Id = :accountId][0].PublicComplaintAccount.size());
            }
            if (shape == 'individual_applications_empty') {
                return String.valueOf([SELECT Id, (SELECT Id FROM IndividualApplicationAccount) FROM Account WHERE Id = :accountId][0].IndividualApplicationAccount.size());
            }
            if (shape == 'regulatory_fees') {
                insert new RegulatoryTrxnFee(AccountId = accountId, Status = 'DRAFT');
                return String.valueOf([SELECT Id, (SELECT Id FROM RegulatoryTrxnFees) FROM Account WHERE Id = :accountId][0].RegulatoryTrxnFees.size());
            }
            if (shape == 'license_applications_empty') {
                return String.valueOf([SELECT Id, (SELECT Id FROM BusinessLicenseApplicationAccount) FROM Account WHERE Id = :accountId][0].BusinessLicenseApplicationAccount.size());
            }
            if (shape == 'applicants_empty') {
                return String.valueOf([SELECT Id, (SELECT Id FROM AccountApplicants) FROM Account WHERE Id = :accountId][0].AccountApplicants.size());
            }
        } catch (Exception error) {
            return error.getTypeName() + '|' + error.getMessage();
        }
        return 'unknown';
    }

    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        Account owner = new Account(Name = 'Owned meadow');
        insert owner;
        Set<String> relationshipNames = new Set<String>();
        for (Schema.ChildRelationship child : Account.SObjectType.getDescribe().getChildRelationships()) {
            if (child.getRelationshipName() != null) relationshipNames.add(child.getRelationshipName().toLowerCase());
        }
        for (String name : new List<String>{'AccountApplicants', 'ApplicationForms', 'BusinessLicenseApplicationAccount', 'BusinessProfiles', 'IndividualApplicationAccount', 'PublicComplaintAccount', 'RegulatoryTrxnFees', 'Contacts', 'Cases'}) {
            result.put('describe_' + name, relationshipNames.contains(name.toLowerCase()));
        }
        for (String shape : new List<String>{'application_forms_empty', 'business_profiles', 'public_complaints', 'individual_applications_empty', 'regulatory_fees', 'license_applications_empty', 'applicants_empty'}) {
            result.put('subquery_' + shape, attempt(owner.Id, shape));
        }
        // Standard field with defaultedOnCreate and calculated=true, createable=false.
        try {
            RegulatoryAuthority authority = new RegulatoryAuthority(Name = 'Owned authority');
            insert authority;
            RegulatoryCode code = new RegulatoryCode(Name = 'Owned code', RegulatoryAuthorityId = authority.Id, Subject = 'Owned subject', EffectiveFrom = Datetime.newInstanceGmt(2026, 1, 2, 3, 4, 5));
            insert code;
            RegulatoryCode loaded = [SELECT Id, IsActive FROM RegulatoryCode WHERE Id = :code.Id];
            result.put('regulatory_code_is_active', loaded.IsActive);
            result.put('regulatory_code_active_filter', [SELECT COUNT() FROM RegulatoryCode WHERE Id = :code.Id AND IsActive = true]);
        } catch (Exception error) {
            result.put('regulatory_code_is_active', error.getTypeName() + '|' + error.getMessage());
        }
        try {
            RegulatoryAuthority windowAuthority = new RegulatoryAuthority(Name = 'Owned window authority');
            insert windowAuthority;
            Datetime now = Datetime.now();
            Map<String, List<Datetime>> windows = new Map<String, List<Datetime>>{
                'past_open' => new List<Datetime>{now.addDays(-2), null},
                'future_open' => new List<Datetime>{now.addDays(2), null},
                'past_past' => new List<Datetime>{now.addDays(-4), now.addDays(-2)},
                'past_future' => new List<Datetime>{now.addDays(-2), now.addDays(2)},
                'future_future' => new List<Datetime>{now.addDays(2), now.addDays(4)}
            };
            for (String key : windows.keySet()) {
                RegulatoryCode windowed = new RegulatoryCode(Name = key, RegulatoryAuthorityId = windowAuthority.Id, Subject = key, EffectiveFrom = windows.get(key)[0], EffectiveTo = windows.get(key)[1]);
                insert windowed;
                result.put('regulatory_code_window_' + key, [SELECT IsActive FROM RegulatoryCode WHERE Id = :windowed.Id].IsActive);
            }
            try {
                insert new RegulatoryCode(Name = 'nofrom', RegulatoryAuthorityId = windowAuthority.Id, Subject = 'nofrom');
                result.put('regulatory_code_no_from', 'inserted');
            } catch (DmlException error) {
                result.put('regulatory_code_no_from', error.getDmlStatusCode(0));
            }
        } catch (Exception error) {
            result.put('regulatory_code_window_error', error.getTypeName() + '|' + error.getMessage());
        }
        Schema.DescribeFieldResult active = RegulatoryCode.IsActive.getDescribe();
        result.put('regulatory_code_describe', active.isCreateable() + ',' + active.isUpdateable() + ',' + active.isCalculated() + ',' + active.isDefaultedOnCreate());
        return result;
    }
}
