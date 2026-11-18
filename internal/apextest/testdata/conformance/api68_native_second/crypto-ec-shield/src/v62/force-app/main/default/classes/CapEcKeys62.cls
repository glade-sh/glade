public class CapEcKeys62 {
    public static void run() {
        List<String> aliases = new List<String>{'ES256','EC-P256','P-256',
            'ES384','EC-P384','P-384','ES512','EC-P521','P-521'};
        for (Integer i = 0; i < aliases.size(); i++) {
            String signAlg = i < 3 ? 'ES256' : (i < 6 ? 'ES384' : 'ES512');
            try {
                Blob pair = Crypto.generateKeyPair(aliases[i]);
                Blob pri = Crypto.getPrivateKey(pair);
                Blob pub = Crypto.getPublicKey(pair);
                Blob msg = Blob.valueOf('S68');
                Blob sig = Crypto.sign(signAlg, msg, pri);
                System.debug(LoggingLevel.ERROR, aliases[i]
                    + ';PAIR_BYTES=' + pair.size()
                    + ';PRIVATE=' + (pri.size() > 0) + ';PRIV_BYTES=' + pri.size()
                    + ';PUBLIC=' + (pub.size() > 0) + ';PUB_BYTES=' + pub.size()
                    + ';SIGNATURE=' + (sig.size() > 0) + ';SIG_BYTES=' + sig.size()
                    + ';VERIFY=' + Crypto.verify(signAlg, msg, sig, pub)
                    + ';ALTERED=' + Crypto.verify(signAlg, Blob.valueOf('S68-changed'), sig, pub));
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, aliases[i] + ';ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
    }
}
