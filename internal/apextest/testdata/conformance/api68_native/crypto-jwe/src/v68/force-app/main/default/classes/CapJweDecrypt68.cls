public class CapJweDecrypt68 {
    public static void run() {
        Blob dummy = Blob.valueOf('dummy');
        try {
            Blob clear = Crypto.decrypt('ECDH-ES+A128KW', 'A128GCM', dummy, dummy,
                dummy, dummy, dummy, dummy, dummy);
            System.debug(LoggingLevel.ERROR, 'CLEAR_BYTES=' + clear.size());
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'DECRYPT_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
