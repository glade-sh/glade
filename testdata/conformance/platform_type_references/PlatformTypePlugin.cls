global class PlatformTypePlugin extends Auth.AuthProviderPluginClass {
    global String getCustomMetadataType() { return null; }
    global PageReference initiate(Map<String,String> config, String state) { return null; }
    global Auth.AuthProviderTokenResponse handleCallback(Map<String,String> config, Auth.AuthProviderCallbackState state) { return null; }
    global Auth.UserData getUserInfo(Map<String,String> config, Auth.AuthProviderTokenResponse response) { return null; }
}
