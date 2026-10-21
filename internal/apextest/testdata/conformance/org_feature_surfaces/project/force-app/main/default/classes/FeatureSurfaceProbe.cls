public class FeatureSurfaceProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        for (String path : new List<String>{'Account.BillingCountryCode','Account.BillingStateCode','Account.ShippingCountryCode','Contact.MailingCountryCode','Lead.CountryCode','User.CountryCode','Order.ShippingCountryCode','Contact.IsPersonAccount','Opportunity.SyncedQuoteId','Case.EntitlementId','Case.BusinessHoursId','Case.MilestoneStatus','Case.SlaStartDate','Case.SlaExitDate','Case.IsStopped','Organization.IsMultiCurrencyEnabled','Organization.IsChatterEnabled','Organization.IsLightningExperienceEnabled','Organization.IsSetPasswordInApiEnabled','Organization.IsHealthCloudEnabled','Organization.HasAnalyticsAdminPerms'}) {
            List<String> parts=path.split('\\.');
            Schema.SObjectType token=Schema.getGlobalDescribe().get(parts[0]);
            result.put(path,token.getDescribe().fields.getMap().containsKey(parts[1]));
        }
        Account item=new Account();
        try {item.put('BillingCountryCode','US');result.put('countryPut',item.get('BillingCountryCode'));}
        catch(Exception ex) {result.put('countryPut',ex.getTypeName());}
        return result;
    }
}
