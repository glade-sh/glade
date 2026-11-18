public class CapJweEncrypt67 {
    public static void run() {
        Blob dummy = Blob.valueOf('dummy');
        Blob cek = Crypto.generateAesKey(128);
        Blob iv = Crypto.generateAesKey(128);
        for (Boolean order : new List<Boolean>{true, false}) {
            try {
                Blob compact = Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM',
                    dummy, order ? dummy : dummy, order ? dummy : dummy,
                    cek, iv, Blob.valueOf('S68'), Blob.valueOf(''));
                System.debug(LoggingLevel.ERROR, 'ORDER=' + order + ';BYTES=' + compact.size());
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, 'ORDER=' + order + ';ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
        try {
            Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM', dummy, dummy, dummy,
                cek, iv, Blob.valueOf('S68'), Blob.valueOf('nonempty-aad'));
            System.debug(LoggingLevel.ERROR, 'AAD=SUCCESS');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'AAD_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
