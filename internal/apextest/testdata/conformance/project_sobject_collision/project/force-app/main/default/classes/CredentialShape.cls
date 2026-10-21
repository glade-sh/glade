public class CredentialShape extends Auth.AuthProviderPluginClass {
    public Account account;

    public CredentialShape() {
        account = new Account('credential');
    }

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

    public String read() {
        Account value = account;
        return value.sfObject;
    }

    public Auth.AuthProviderTokenResponse handleCallback(
        Map<String, String> config,
        Auth.AuthProviderCallbackState state
    ) {
        return new Auth.AuthProviderTokenResponse(
            config.get('provider'),
            account.sfObject,
            '',
            state.queryParameters.get('state')
        );
    }
}
