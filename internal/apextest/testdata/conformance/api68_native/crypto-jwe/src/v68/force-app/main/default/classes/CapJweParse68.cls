public class CapJweParse68 {
    public static void run() {
        for (String tok : new List<String>{'a.b.c', 'a.b.c.d.e', 'a.b.c.d.e.f', ''}) {
            try {
                System.JweToken t = Crypto.parseJwe(Blob.valueOf(tok));
                System.debug(LoggingLevel.ERROR, 'TOKEN=' + tok + ';ALG=' + t.getAlgorithm());
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, 'TOKEN=' + tok + ';ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
        try {
            Blob big = Blob.valueOf('a'.repeat(1048577));
            Crypto.parseJwe(big);
            System.debug(LoggingLevel.ERROR, 'BIG=SUCCESS');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'BIG=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
