@isTest
global class ResourcesLabelsTest {
  @isTest global static void labelsResolveFromMetadata() {
    System.assertEquals('Hello', Label.Greeting);
    System.assertEquals('Hello', System.Label.Greeting);
    System.assertEquals('Hello', Label.pkg.Greeting);
  }

  @isTest global static void staticResourceMockUsesNamedCredentialEndpoint() {
    MultiStaticResourceCalloutMock multiMock = new MultiStaticResourceCalloutMock();
    multiMock.setStaticResource('https://billing.example.test/v1/accounts', 'Site');
    Test.setMock('HttpCalloutMock', multiMock);

    HttpRequest req = new HttpRequest();
    req.setEndpoint('callout:Billing/v1/accounts');
    req.setMethod('GET');
    System.assertEquals('site-body\n', new Http().send(req).getBody());
  }
}
