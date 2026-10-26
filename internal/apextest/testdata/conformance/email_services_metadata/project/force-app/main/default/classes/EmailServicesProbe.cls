public class EmailServicesProbe {
    public static Object observe(String shape) {
        if (shape == 'missing') {
            try { EmailServicesAddress a = [SELECT LocalPart FROM EmailServicesAddress WHERE LocalPart = 'absent_svc']; return a.LocalPart; }
            catch (Exception e) { return new Map<String,Object>{'exceptionType'=>e.getTypeName(),'exceptionMessage'=>e.getMessage()}; }
        }
        String part = shape == 'inactive' ? 'inactive_svc' : (shape == 'missing' ? 'absent_svc' : 'inbound_svc');
        List<EmailServicesAddress> records;
        if (shape == 'limited') records = [SELECT DeveloperName, LocalPart, EmailDomainName, IsActive, AuthorizedSenders, FunctionId, Function.FunctionName, Function.IsActive, RunAsUserId FROM EmailServicesAddress ORDER BY DeveloperName LIMIT 1];
        else records = [SELECT DeveloperName, LocalPart, EmailDomainName, IsActive, AuthorizedSenders, FunctionId, Function.FunctionName, Function.IsActive, RunAsUserId FROM EmailServicesAddress WHERE LocalPart = :part ORDER BY DeveloperName];
        List<Object> out = new List<Object>();
        for (EmailServicesAddress a : records) {
            String domain = a.EmailDomainName;
            List<String> labels = domain.split('\\.');
            List<Integer> lengths = new List<Integer>();
            for (String label : labels) lengths.add(label.length());
            out.add(new Map<String,Object>{'developerName'=>a.DeveloperName,'localPart'=>a.LocalPart,'active'=>a.IsActive,'authorizedSenders'=>a.AuthorizedSenders,'functionName'=>a.Function.FunctionName,'functionActive'=>a.Function.IsActive,'functionPresent'=>a.FunctionId != null,'runAsCurrentUser'=>a.RunAsUserId == UserInfo.getUserId(),'domainSuffix'=>domain.substring(domain.indexOf('.apex.')),'domainLabelLengths'=>lengths,'domainLowercase'=>domain == domain.toLowerCase()});
        }
        return out;
    }
}
