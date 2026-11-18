public class CapJweMatrix68 {
    public static void run(String enc, Integer cekBytes) {
        List<String> wraps = new List<String>{
            'ECDH-ES+A128KW', 'ECDH-ES+A192KW', 'ECDH-ES+A256KW'};
        for (String alg : wraps) {
            for (Boolean signatureOrder : new List<Boolean>{true, false}) {
                String tag = alg + '/' + enc + ';ORDER=' + signatureOrder;
                try {
                    Blob rp = Crypto.generateKeyPair('P-256');
                    Blob ep = Crypto.generateKeyPair('P-256');
                    Blob rpub = Crypto.getPublicKey(rp);
                    Blob rpri = Crypto.getPrivateKey(rp);
                    Blob epub = Crypto.getPublicKey(ep);
                    Blob epri = Crypto.getPrivateKey(ep);
                    String randomHex = EncodingUtil.convertToHex(Crypto.generateAesKey(256))
                        + EncodingUtil.convertToHex(Crypto.generateAesKey(256));
                    Blob cek = EncodingUtil.convertFromHex(randomHex.substring(0, cekBytes * 2));
                    String ivHex = EncodingUtil.convertToHex(Crypto.generateAesKey(128));
                    Blob iv = EncodingUtil.convertFromHex(
                        ivHex.substring(0, enc.endsWith('GCM') ? 24 : 32));
                    Blob compact = Crypto.encrypt(alg, enc, rpub,
                        signatureOrder ? epub : epri, signatureOrder ? epri : epub,
                        cek, iv, Blob.valueOf('S68'), Blob.valueOf(''));
                    System.JweToken token = Crypto.parseJwe(compact);
                    Blob clear = Crypto.decrypt(token.getAlgorithm(),
                        token.getEncryptionAlgorithm(), rpri,
                        token.getEphemeralPublicKey(), token.getEncryptedKey(),
                        token.getIv(), token.getCiphertext(), token.getTag(),
                        token.getAad());
                    String protectedHeader = compact.toString().split('\\.')[0];
                    System.JweToken copy = new System.JweToken(token.getAlgorithm(),
                        token.getEncryptionAlgorithm(), token.getEphemeralPublicKey(),
                        token.getEncryptedKey(), token.getIv(), token.getCiphertext(),
                        token.getTag(), token.getAad());
                    String expected = 'System.JweToken[alg=' + token.getAlgorithm()
                        + ', enc=' + token.getEncryptionAlgorithm()
                        + ', epkLen=' + token.getEphemeralPublicKey().size()
                        + ', encKeyLen=' + token.getEncryptedKey().size()
                        + ', ivLen=' + token.getIv().size()
                        + ', ciphertextLen=' + token.getCiphertext().size()
                        + ', tagLen=' + token.getTag().size()
                        + ', aadLen=' + token.getAad().size() + ']';
                    System.debug(LoggingLevel.ERROR, tag
                        + ';ROUNDTRIP=' + (clear.toString() == 'S68')
                        + ';AAD_HEADER=' + (token.getAad().toString() == protectedHeader)
                        + ';JWE_COPY=' + (copy.toString() == token.toString())
                        + ';JWE_STRING_MATCH=' + (copy.toString() == expected
                            && token.toString() == expected)
                        + ';AAD_NONNULL=' + (token.getAad() != null)
                        + ';WRAPPED_KEY_NONEMPTY=' + (token.getEncryptedKey() != null
                            && token.getEncryptedKey().size() > 0)
                        + ';IV_BYTES=' + token.getIv().size()
                        + ';TAG_BYTES=' + token.getTag().size()
                        + ';CEK_BYTES=' + cek.size()
                        + ';TOKEN_BYTES=' + compact.size());
                } catch (Exception e) {
                    System.debug(LoggingLevel.ERROR, tag + ';ERR=' + e.getTypeName()
                        + ': ' + e.getMessage());
                }
            }
        }
    }
}
