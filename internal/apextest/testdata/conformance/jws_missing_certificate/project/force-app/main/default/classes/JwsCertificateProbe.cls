public class JwsCertificateProbe {
    static String describe(Exception issue) {
        String cause = issue.getCause() == null ? 'nocause' : issue.getCause().getTypeName();
        String text = issue.getMessage();
        if (text != null && text.length() > 40) text = text.substring(0, 40);
        return issue.getTypeName() + ':' + text + ':' + cause;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        if (!Test.isRunningTest()) { result.put('context', 'anonymous'); return result; }
        Auth.JWT jwt = new Auth.JWT();
        jwt.setSub('probe@example.com');
        jwt.setAud('https://login.salesforce.com');
        jwt.setIss('client');
        jwt.setValidityLength(600);
        Auth.JWS jws = null;
        try { jws = new Auth.JWS(jwt, 'No_Such_Certificate'); result.put('construct', 'ok'); } catch (Exception issue) { result.put('construct', describe(issue)); }
        if (jws != null) {
            try { result.put('compact', String.valueOf(jws.getCompactSerialization() != null)); } catch (Exception issue) { result.put('compact', describe(issue)); }
            try {
                Auth.JWTBearerTokenExchange exchange = new Auth.JWTBearerTokenExchange('https://login.salesforce.com/services/oauth2/token', jws);
                result.put('exchange', 'constructed');
                HttpResponse response = exchange.getHttpResponse();
                result.put('exchangeResponse', response == null ? 'null' : String.valueOf(response.getStatusCode()));
            } catch (Exception issue) { result.put('exchange', describe(issue)); }
        }
        try { Crypto.signWithCertificate('RSA-SHA256', Blob.valueOf('x'), 'No_Such_Certificate'); result.put('sign', 'ok'); } catch (Exception issue) { result.put('sign', describe(issue)); }
        try { Auth.JWTBearerTokenExchange missing = new Auth.JWTBearerTokenExchange('https://login.salesforce.com/services/oauth2/token', null); result.put('nullJws', 'ok'); } catch (Exception issue) { result.put('nullJws', describe(issue)); }
        Auth.AuthProviderPluginException pluginIssue = new Auth.AuthProviderPluginException('outer');
        pluginIssue.initCause(new NoDataFoundException());
        result.put('initCause', pluginIssue.getCause().getTypeName() + ':' + pluginIssue.getCause().getMessage());
        return result;
    }
}
