public class RunAsIdentity {
    // Fingerprints are compared only in the capture process. Exported native
    // rows contain equality booleans, never fingerprints or full domains.
    public static Object observe() {
        Map<String,String> result = new Map<String,String>();
        for (EmailServicesAddress row : [SELECT DeveloperName, EmailDomainName FROM EmailServicesAddress ORDER BY DeveloperName]) {
            result.put(row.DeveloperName, EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(row.EmailDomainName))));
        }
        return result;
    }
}
