@IsTest private class PlatformTypeReferencesTest {
    public Schema.AggregateResult field0;
    public Auth.AuthProviderPluginClass field1;
    public Auth.AuthProviderPluginException field2;
    public Auth.JWTValidationException field3;
    public Auth.AuthProviderTokenResponse field4;
    public Auth.UserData field5;
    public Auth.AuthProviderCallbackState field6;
    public Auth.OAuthRefreshResult field7;
    public Auth.JWTBearerTokenExchange field8;
    public Auth.JWS field9;
    public Auth.RegistrationHandler field10;
    public Cache.Org.OrgCacheException field11;
    public Schema.AggregateResult echo0(Schema.AggregateResult value) { return value; }
    public Auth.AuthProviderPluginClass echo1(Auth.AuthProviderPluginClass value) { return value; }
    public Auth.AuthProviderPluginException echo2(Auth.AuthProviderPluginException value) { return value; }
    public Auth.JWTValidationException echo3(Auth.JWTValidationException value) { return value; }
    public Auth.AuthProviderTokenResponse echo4(Auth.AuthProviderTokenResponse value) { return value; }
    public Auth.UserData echo5(Auth.UserData value) { return value; }
    public Auth.AuthProviderCallbackState echo6(Auth.AuthProviderCallbackState value) { return value; }
    public Auth.OAuthRefreshResult echo7(Auth.OAuthRefreshResult value) { return value; }
    public Auth.JWTBearerTokenExchange echo8(Auth.JWTBearerTokenExchange value) { return value; }
    public Auth.JWS echo9(Auth.JWS value) { return value; }
    public Auth.RegistrationHandler echo10(Auth.RegistrationHandler value) { return value; }
    public Cache.Org.OrgCacheException echo11(Cache.Org.OrgCacheException value) { return value; }
    @IsTest static void observe() {
Schema.AggregateResult value0 = null; List<Schema.AggregateResult> values0 = new List<Schema.AggregateResult>();
Auth.AuthProviderPluginClass value1 = null; List<Auth.AuthProviderPluginClass> values1 = new List<Auth.AuthProviderPluginClass>();
Auth.AuthProviderPluginException value2 = null; List<Auth.AuthProviderPluginException> values2 = new List<Auth.AuthProviderPluginException>();
Auth.JWTValidationException value3 = null; List<Auth.JWTValidationException> values3 = new List<Auth.JWTValidationException>();
Auth.AuthProviderTokenResponse value4 = null; List<Auth.AuthProviderTokenResponse> values4 = new List<Auth.AuthProviderTokenResponse>();
Auth.UserData value5 = null; List<Auth.UserData> values5 = new List<Auth.UserData>();
Auth.AuthProviderCallbackState value6 = null; List<Auth.AuthProviderCallbackState> values6 = new List<Auth.AuthProviderCallbackState>();
Auth.OAuthRefreshResult value7 = null; List<Auth.OAuthRefreshResult> values7 = new List<Auth.OAuthRefreshResult>();
Auth.JWTBearerTokenExchange value8 = null; List<Auth.JWTBearerTokenExchange> values8 = new List<Auth.JWTBearerTokenExchange>();
Auth.JWS value9 = null; List<Auth.JWS> values9 = new List<Auth.JWS>();
Auth.RegistrationHandler value10 = null; List<Auth.RegistrationHandler> values10 = new List<Auth.RegistrationHandler>();
Cache.Org.OrgCacheException value11 = null; List<Cache.Org.OrgCacheException> values11 = new List<Cache.Org.OrgCacheException>();
List<Schema.AggregateResult> r = new List<Schema.AggregateResult>();
for (Schema.AggregateResult a : [SELECT COUNT(Id) c FROM Account GROUP BY Name]) {}
try { Integer value = 1; } catch (Auth.AuthProviderPluginException ex) {}
try { Integer value = 1; } catch (Auth.JWTValidationException ex) {}

        PlatformTypeReferences.observe();
        PlatformTypePlugin plugin = new PlatformTypePlugin();
        PlatformTypeRegistration registration = new PlatformTypeRegistration();
        System.assertNotEquals(null, plugin);
        System.assertNotEquals(null, registration);
    }
}
