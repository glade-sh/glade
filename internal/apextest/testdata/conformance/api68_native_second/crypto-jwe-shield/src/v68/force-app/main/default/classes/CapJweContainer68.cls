public class CapJweContainer68 {
    public static void run() {
        Blob epk = Blob.valueOf('ek');
        Blob iv = EncodingUtil.convertFromHex('000000000000000000000000');
        Blob tag = Crypto.generateAesKey(128);
        Blob aad = Blob.valueOf('aad');
        try {
            System.JweToken t = new System.JweToken('ECDH-ES+A128KW', 'A128GCM', epk,
                Blob.valueOf('k'), iv, Blob.valueOf('ct'), tag, aad);
            System.debug(LoggingLevel.ERROR, 'ALG=' + t.getAlgorithm()
                + ';ENC=' + t.getEncryptionAlgorithm()
                + ';EPK=' + (t.getEphemeralPublicKey() == epk)
                + ';EK=' + (t.getEncryptedKey() != null)
                + ';IV=' + (t.getIv() == iv)
                + ';CT=' + (t.getCiphertext() != null)
                + ';TAG=' + (t.getTag() == tag)
                + ';AAD=' + (t.getAad() == aad));
            System.debug(LoggingLevel.ERROR, 'STR=' + t.toString());
            System.JweToken copy = new System.JweToken(t.getAlgorithm(),
                t.getEncryptionAlgorithm(), t.getEphemeralPublicKey(), t.getEncryptedKey(),
                t.getIv(), t.getCiphertext(), t.getTag(), t.getAad());
            System.debug(LoggingLevel.ERROR, 'COPY=' + (copy.toString() == t.toString())
                + ';COPY_STR=' + copy.toString());
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'CTOR_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            System.JweToken bad = new System.JweToken('', '', Blob.valueOf(''),
                Blob.valueOf(''), Blob.valueOf(''), Blob.valueOf(''), Blob.valueOf(''), Blob.valueOf(''));
            System.debug(LoggingLevel.ERROR, 'EMPTY_CTOR=OK;STR=' + bad.toString());
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'EMPTY_CTOR_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
