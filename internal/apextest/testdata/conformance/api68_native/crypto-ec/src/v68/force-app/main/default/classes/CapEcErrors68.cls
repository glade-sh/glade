public class CapEcErrors68 {
    public static void run() {
        for (String mode : new List<String>{'ALIAS','SHORT','BAD_LENGTH','NO_PUBLIC','PARTS'}) {
            try {
                if (mode == 'ALIAS') {
                    Crypto.generateKeyPair('S68_NOT_AN_ALGORITHM');
                }
                if (mode == 'SHORT') {
                    Crypto.getPrivateKey(EncodingUtil.convertFromHex('000000'));
                }
                if (mode == 'BAD_LENGTH') {
                    Crypto.getPrivateKey(EncodingUtil.convertFromHex('7fffffff00'));
                }
                if (mode == 'NO_PUBLIC') {
                    Blob pair = Crypto.generateKeyPair('P-256');
                    Blob pri = Crypto.getPrivateKey(pair);
                    String hexPair = EncodingUtil.convertToHex(pair);
                    Blob noPublic = EncodingUtil.convertFromHex(
                        hexPair.substring(0, (4 + pri.size()) * 2));
                    Crypto.getPublicKey(noPublic);
                }
                if (mode == 'PARTS') {
                    Crypto.parseJwe(Blob.valueOf('a.b.c'));
                }
                System.debug(LoggingLevel.ERROR, mode + '=SUCCESS');
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, mode + '=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
    }
}
