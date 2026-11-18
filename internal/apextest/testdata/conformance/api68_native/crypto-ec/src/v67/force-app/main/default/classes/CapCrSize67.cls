public class CapCrSize67 {
    public static void run() {
        Blob key = Crypto.generateAesKey(256);
        Blob iv = Crypto.generateAesKey(128);
        for (Integer bytes : new List<Integer>{1048576, 1048577}) {
            Blob input = Blob.valueOf('x'.repeat(bytes));
            try {
                Blob out = Crypto.encrypt('AES256', key, iv, input);
                System.debug(LoggingLevel.ERROR, 'SIZE=' + bytes + ';OUT_BYTES=' + out.size());
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, 'SIZE=' + bytes + ';ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
    }
}
