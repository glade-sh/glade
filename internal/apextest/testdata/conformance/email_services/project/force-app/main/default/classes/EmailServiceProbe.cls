public class EmailServiceProbe {
    public static Map<String,String> observe() {
        Map<String,String> rows = new Map<String,String>();
        List<EmailServicesFunction> functions = [SELECT Id, FunctionName, IsActive, ApexClassId, AttachmentOption, AuthenticationFailureAction, AuthorizationFailureAction, AddressInactiveAction, FunctionInactiveAction, OverLimitAction, AuthorizedSenders, ErrorRoutingAddress, IsAuthenticationRequired, IsErrorRoutingEnabled, IsTextAttachmentsAsBinary, IsTlsRequired FROM EmailServicesFunction WHERE FunctionName IN ('MailReceiver','MailDormant') ORDER BY FunctionName];
        rows.put('E001', String.valueOf(functions.size()));
        for (EmailServicesFunction f : functions) {
            ApexClass handler = [SELECT Name FROM ApexClass WHERE Id = :f.ApexClassId];
            rows.put(f.FunctionName, f.FunctionName+'|'+f.IsActive+'|'+handler.Name+'|'+f.AttachmentOption+'|'+f.AuthenticationFailureAction+'|'+f.AuthorizationFailureAction+'|'+f.AddressInactiveAction+'|'+f.FunctionInactiveAction+'|'+f.OverLimitAction+'|'+f.AuthorizedSenders+'|'+f.ErrorRoutingAddress+'|'+f.IsAuthenticationRequired+'|'+f.IsErrorRoutingEnabled+'|'+f.IsTextAttachmentsAsBinary+'|'+f.IsTlsRequired);
        }
        List<EmailServicesAddress> addresses = [SELECT Id,DeveloperName,LocalPart,EmailDomainName,IsActive,AuthorizedSenders,FunctionId,RunAsUserId FROM EmailServicesAddress ORDER BY DeveloperName];
        rows.put('E002', String.valueOf(addresses.size()));
        Set<String> domains = new Set<String>();
        for (EmailServicesAddress a : addresses) {
            EmailServicesFunction f = [SELECT FunctionName FROM EmailServicesFunction WHERE Id = :a.FunctionId];
            Boolean shape = a.EmailDomainName != null && Pattern.matches('[a-z0-9-]+\\.[a-z0-9-]+\\.[a-z0-9]+\\.apex\\.[a-z0-9]+\\.salesforce\\.com', a.EmailDomainName);
            rows.put(a.DeveloperName, a.LocalPart+'|'+a.IsActive+'|'+a.AuthorizedSenders+'|'+f.FunctionName+'|'+(a.RunAsUserId == UserInfo.getUserId())+'|'+shape);
            domains.add(a.EmailDomainName);
        }
        rows.put('E003', String.valueOf(domains.size()));
        rows.put('E004', String.valueOf([SELECT COUNT() FROM EmailServicesAddress WHERE LocalPart='does_not_exist']));
        try { new EmailServicesAddress().put('EmailDomainName','example.invalid'); rows.put('E005','accepted'); }
        catch (Exception e) { rows.put('E005',e.getTypeName()+':'+e.getMessage()); }
        return rows;
    }
}
