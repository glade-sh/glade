public class TokenProviderProbe extends Auth.AuthProviderPluginClass {
    public String getCustomMetadataType() {
        return 'TokenConfig__mdt';
    }

    public PageReference initiate(Map<String, String> config, String stateToPropagate) {
        return new PageReference('/');
    }

    public Auth.UserData getUserInfo(
        Map<String, String> config,
        Auth.AuthProviderTokenResponse response
    ) {
        return new Auth.UserData(
            'id', null, null, null, null, null, null, null, null, null,
            new Map<String, String>()
        );
    }

    public String algorithmForSample() {
        List<TokenConfig__mdt> rows = [
            SELECT Algorithm__c
            FROM TokenConfig__mdt
            WHERE DeveloperName = 'Sample'
            LIMIT 1
        ];
        JWT jwt = new JWT(rows[0].Algorithm__c);
        return jwt.algorithm;
    }

    public Auth.AuthProviderTokenResponse handleCallback(
        Map<String, String> config,
        Auth.AuthProviderCallbackState state
    ) {
        List<TokenConfig__mdt> rows = [
            SELECT Algorithm__c
            FROM TokenConfig__mdt
            WHERE DeveloperName = 'Sample'
            LIMIT 1
        ];
        JWT jwt = new JWT(rows[0].Algorithm__c);
        return new Auth.AuthProviderTokenResponse(
            config.get('provider'),
            jwt.algorithm,
            '',
            state.queryParameters.get('state')
        );
    }
}
