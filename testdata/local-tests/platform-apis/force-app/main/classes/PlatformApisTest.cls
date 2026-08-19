@isTest
private class PlatformApisTest {
  @isTest static void callableAndStubProviderWorkInTests() {
    System.Callable callable = new LocalCallable();
    Map<String, Object> args = new Map<String, Object>();
    args.put('value', 'local');
    System.assert(callable instanceof System.Callable);
    System.assertEquals('local', (String) callable.call('echo', args));

    Greeter greeter = Test.createStub(Greeter.class, new GreeterProvider());
    System.assertEquals('stubbed', greeter.greet('Ada'));
  }

  @isTest static void contextAndOrganizationApisUseLocalContracts() {
    System.assertEquals('local-site', Site.getSiteId());
    System.assertEquals(false, Auth.CommunitiesUtil.isGuestUser());

    ConnectApi.OrganizationSettings settings = ConnectApi.Organization.getSettings();
    System.assertEquals('00D000000000001', settings.orgId);
  }

  @isTest static void platformCachePartitionsAreIsolatedAndMutable() {
    Cache.OrgPartition orgCache = Cache.Org.getPartition('local');
    System.assertEquals(null, orgCache.get('missing'));
    orgCache.put('name', 'Acme');
    System.assert(orgCache.contains('name'));
    System.assertEquals('Acme', (String) orgCache.get('name'));
    System.assertEquals('Acme', String.valueOf(orgCache.remove('name')));
    System.assertEquals(false, orgCache.contains('name'));

    Cache.SessionPartition sessionCache = Cache.Session.getPartition('local');
    sessionCache.put('count', 7, 60);
    System.assertEquals(7, (Integer) sessionCache.get('count'));
  }

  @isTest static void namedCredentialEndpointResolvesForCalloutMocks() {
    MultiStaticResourceCalloutMock mock = new MultiStaticResourceCalloutMock();
    mock.setStaticResource('https://billing.example.test/v1/accounts', 'Api_Response');
    Test.setMock('HttpCalloutMock', mock);

    HttpRequest req = new HttpRequest();
    req.setEndpoint('callout:Billing/v1/accounts');
    req.setMethod('GET');
    System.assertEquals('api-body\n', new Http().send(req).getBody());
  }
}
