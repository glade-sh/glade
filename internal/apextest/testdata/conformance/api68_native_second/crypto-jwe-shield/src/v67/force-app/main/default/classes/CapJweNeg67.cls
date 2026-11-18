public class CapJweNeg67 {
    public static void run() {
        Blob rp = Crypto.generateKeyPair('P-256');
        Blob rpub = Crypto.getPublicKey(rp);
        Blob rpri = Crypto.getPrivateKey(rp);
        Blob ep = Crypto.generateKeyPair('P-256');
        Blob epub = Crypto.getPublicKey(ep);
        Blob epri = Crypto.getPrivateKey(ep);
        Blob cek = Crypto.generateAesKey(128);
        Blob iv12 = EncodingUtil.convertFromHex(
            EncodingUtil.convertToHex(Crypto.generateAesKey(128)).substring(0, 24));
        Blob iv16 = Crypto.generateAesKey(128);
        Blob plain = Blob.valueOf('S68');
        System.JweToken token = null;
        try {
            Blob compact = Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM', rpub, epri, epub,
                cek, iv12, plain, Blob.valueOf(''));
            token = Crypto.parseJwe(compact);
            System.debug(LoggingLevel.ERROR, 'BASE=OK;TOKEN_BYTES=' + compact.size());
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'BASE_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM', rpub, epri, epub,
                cek, iv12, plain, Blob.valueOf('nonempty'));
            System.debug(LoggingLevel.ERROR, 'AAD=SUCCESS');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'AAD_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            Blob cek32 = Crypto.generateAesKey(256);
            Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM', rpub, epri, epub,
                cek32, iv12, plain, Blob.valueOf(''));
            System.debug(LoggingLevel.ERROR, 'CEK_MISMATCH=SUCCESS');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'CEK_MISMATCH_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            Blob c16 = Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM', rpub, epri, epub,
                cek, iv16, plain, Blob.valueOf(''));
            System.JweToken t16 = Crypto.parseJwe(c16);
            Blob p16 = Crypto.decrypt(t16.getAlgorithm(), t16.getEncryptionAlgorithm(),
                rpri, t16.getEphemeralPublicKey(), t16.getEncryptedKey(), t16.getIv(),
                t16.getCiphertext(), t16.getTag(), t16.getAad());
            System.debug(LoggingLevel.ERROR, 'IV16=SUCCESS;PARSED_IV='
                + t16.getIv().size() + ';CLEAR=' + p16.toString());
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'IV16_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        try {
            Blob ep384 = Crypto.generateKeyPair('P-384');
            Blob epub384 = Crypto.getPublicKey(ep384);
            Blob epri384 = Crypto.getPrivateKey(ep384);
            Crypto.encrypt('ECDH-ES+A128KW', 'A128GCM', rpub, epub384, epri384,
                cek, iv12, plain, Blob.valueOf(''));
            System.debug(LoggingLevel.ERROR, 'CURVE_MISMATCH=SUCCESS');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'CURVE_MISMATCH_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        if (token != null) {
            try {
                String tagHex = EncodingUtil.convertToHex(token.getTag());
                String flipped = (tagHex.substring(0, 1) == '0' ? '1' : '0')
                    + tagHex.substring(1);
                Crypto.decrypt(token.getAlgorithm(), token.getEncryptionAlgorithm(),
                    rpri, token.getEphemeralPublicKey(), token.getEncryptedKey(),
                    token.getIv(), token.getCiphertext(),
                    EncodingUtil.convertFromHex(flipped), token.getAad());
                System.debug(LoggingLevel.ERROR, 'TAMPER=SUCCESS');
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, 'TAMPER_ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
            try {
                Crypto.decrypt(token.getAlgorithm(), token.getEncryptionAlgorithm(),
                    rpri, token.getEphemeralPublicKey(), Blob.valueOf(''),
                    token.getIv(), token.getCiphertext(), token.getTag(), token.getAad());
                System.debug(LoggingLevel.ERROR, 'EMPTY_EK=SUCCESS');
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, 'EMPTY_EK_ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
            try {
                Blob other = Crypto.generateKeyPair('P-256');
                Crypto.decrypt(token.getAlgorithm(), token.getEncryptionAlgorithm(),
                    Crypto.getPrivateKey(other), token.getEphemeralPublicKey(),
                    token.getEncryptedKey(), token.getIv(), token.getCiphertext(),
                    token.getTag(), token.getAad());
                System.debug(LoggingLevel.ERROR, 'WRONG_KEY=SUCCESS');
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, 'WRONG_KEY_ERR=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
        for (String tok : new List<String>{'a.b.c', 'a.b.c.d.e.f', ''}) {
            try {
                Crypto.parseJwe(Blob.valueOf(tok));
                System.debug(LoggingLevel.ERROR, 'PARSE_' + tok + '=SUCCESS');
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, 'PARSE_' + tok + '=' + e.getTypeName() + ': ' + e.getMessage());
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
