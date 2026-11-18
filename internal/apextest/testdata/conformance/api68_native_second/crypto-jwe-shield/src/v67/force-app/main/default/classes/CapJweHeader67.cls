public class CapJweHeader67 {
    public static void run() {
        List<String> headers = new List<String>{
            '{"alg":"ECDH-ES+A128KW","enc":"A128GCM"}',
            '{"enc":"A128GCM","epk":{"kty":"EC","crv":"P-256","x":"AA","y":"AA"}}',
            '{"alg":"ECDH-ES+A128KW","epk":{"kty":"EC","crv":"P-256","x":"AA","y":"AA"}}',
            '{"alg":"ECDH-ES+A128KW","enc":"A128GCM","epk":{"kty":"RSA","crv":"P-256","x":"AA","y":"AA"}}',
            '{"alg":"ECDH-ES+A128KW","enc":"A128GCM","epk":{"kty":"EC","crv":"P-999","x":"AA","y":"AA"}}',
            '{"alg":"ECDH-ES+A128KW","enc":"A128GCM","epk":{"kty":"EC","x":"AA","y":"AA"}}',
            '{"alg":"ECDH-ES+A128KW","enc":"S68-BAD","epk":{"kty":"EC","crv":"P-256","x":"AA","y":"AA"}}',
            'not-json'
        };
        for (String headerJson : headers) {
            String b64 = EncodingUtil.base64Encode(Blob.valueOf(headerJson));
            String b64url = b64.replace('+', '-').replace('/', '_');
            while (b64url.endsWith('=')) {
                b64url = b64url.substring(0, b64url.length() - 1);
            }
            String compact = b64url + '.AAAA.AAAA.AAAA.AAAA';
            try {
                System.JweToken t = Crypto.parseJwe(Blob.valueOf(compact));
                System.debug(LoggingLevel.ERROR, headerJson + ';PARSED=true;ALG='
                    + t.getAlgorithm() + ';ENC=' + t.getEncryptionAlgorithm());
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, headerJson + ';ERR=' + e.getTypeName()
                    + ': ' + e.getMessage());
            }
        }
    }
}
