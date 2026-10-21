public class FeatureSettingsProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        result.put('multiCurrency',UserInfo.isMultiCurrencyOrganization());
        for (String path : new List<String>{'Account.CurrencyIsoCode','Opportunity.CurrencyIsoCode','User.DefaultCurrencyIsoCode'}) {
            List<String> parts=path.split('\\.');
            Schema.SObjectType token=Schema.getGlobalDescribe().get(parts[0]);
            result.put(path,token.getDescribe().fields.getMap().containsKey(parts[1]));
        }
        return result;
    }
}
