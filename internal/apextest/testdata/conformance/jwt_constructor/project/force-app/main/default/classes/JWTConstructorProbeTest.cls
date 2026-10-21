@IsTest
private class JWTConstructorProbeTest {
    @IsTest
    static void metadataTextArgument() {
        TokenProviderProbe provider = new TokenProviderProbe();
        String algorithm = provider.algorithmForSample();
        System.assert(false, 'OBSERVED|metadataTextArgument|' + JSON.serialize(new Map<String, Object>{'algorithm' => algorithm}));
    }
}
