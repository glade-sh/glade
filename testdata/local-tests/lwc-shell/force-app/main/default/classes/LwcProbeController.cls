public with sharing class LwcProbeController {
    @AuraEnabled(cacheable=true)
    public static List<Account> wireAccounts(String industry) {
        String selectedIndustry = String.isBlank(industry) ? 'Technology' : industry;
        return new List<Account>{
            new Account(Name = 'Local Shell Account', Industry = selectedIndustry),
            new Account(Name = 'Backup Shell Account', Industry = selectedIndustry)
        };
    }

    @AuraEnabled
    public static Account imperativeAccount(String name) {
        String selectedName = String.isBlank(name) ? 'Imperative Shell Account' : name;
        return new Account(Name = selectedName, Industry = 'Services');
    }
}
