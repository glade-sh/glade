public class CapEcRsa67 {
    public static void run() {
        Blob pair = Crypto.generateKeyPair('P-256');
        Blob pri = Crypto.getPrivateKey(pair);
        Blob pub = Crypto.getPublicKey(pair);
        Blob msg = Blob.valueOf('S68');
        for (String alg : new List<String>{'RSA-SHA256','RSA','S68_NOT_AN_ALGORITHM'}) {
            try {
                Blob sig = Crypto.sign(alg, msg, pri);
                System.debug(LoggingLevel.ERROR, alg + ';SIGN_OK=true;SIG_BYTES=' + sig.size());
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, alg + ';SIGN_ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
        try {
            Blob fakeSig = Crypto.generateAesKey(256);
            System.debug(LoggingLevel.ERROR, 'VERIFY_BAD=' + Crypto.verify('ES256', msg, fakeSig, pub));
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'VERIFY_BAD_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
