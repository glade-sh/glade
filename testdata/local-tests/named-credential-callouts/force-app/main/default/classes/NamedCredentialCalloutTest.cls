@isTest
private class NamedCredentialCalloutTest {
  @isTest static void multiEndpointMockResolvesNamedCredentialsAndRemoteUrls() {
    MultiStaticResourceCalloutMock mock = new MultiStaticResourceCalloutMock();
    mock.setStaticResource('https://billing.example.test/v1/accounts', 'Billing_Accounts_Response');
    mock.setStaticResource('https://tax.example.test/v2/rates', 'Tax_Rates_Response');
    mock.setStaticResource('https://maps.example.test/geocode', 'Maps_Response');
    mock.setStatusCode(207);
    mock.setHeader('Content-Type', 'application/json');
    Test.setMock('HttpCalloutMock', mock);

    Http http = new Http();

    HttpRequest billing = new HttpRequest();
    billing.setEndpoint('callout:Billing/v1/accounts');
    billing.setMethod('GET');
    billing.setHeader('X-Correlation-Id', 'billing-001');
    HttpResponse billingResponse = http.send(billing);
    System.assertEquals(207, billingResponse.getStatusCode());
    System.assertEquals('application/json', billingResponse.getHeader('content-type'));
    System.assertEquals('billing-accounts\n', billingResponse.getBody());
    System.assertEquals('callout:Billing/v1/accounts', billing.getEndpoint());
    System.assertEquals('billing-001', billing.getHeader('x-correlation-id'));

    HttpRequest tax = new HttpRequest();
    tax.setEndpoint('callout:pkg__TaxService/v2/rates');
    tax.setMethod('POST');
    tax.setBodyAsBlob(Blob.valueOf('state=CA'));
    HttpResponse taxResponse = http.send(tax);
    System.assertEquals('tax-rates\n', taxResponse.getBody());
    System.assertEquals('state=CA', tax.getBody());

    HttpRequest maps = new HttpRequest();
    maps.setEndpoint('https://maps.example.test/geocode');
    maps.setMethod('GET');
    System.assertEquals('maps-response\n', http.send(maps).getBody());

    System.assertEquals(3, Limits.getCallouts());
  }

  @isTest static void repeatedNamedCredentialCalloutCarriesStatusHeadersAndBody() {
    MultiStaticResourceCalloutMock mock = new MultiStaticResourceCalloutMock();
    mock.setStaticResource('https://billing.example.test/v1/accounts/001', 'Billing_Detail_Response');
    mock.setStatusCode(202);
    mock.setHeader('X-Source', 'fixture');
    Test.setMock('HttpCalloutMock', mock);

    HttpRequest request = new HttpRequest();
    request.setEndpoint('callout:Billing/v1/accounts/001');
    request.setMethod('PATCH');
    request.setBody('{"active":true}');

    HttpResponse response = new Http().send(request);
    System.assertEquals(202, response.getStatusCode());
    System.assertEquals('fixture', response.getHeader('x-source'));
    System.assertEquals('billing-detail\n', response.getBody());
    System.assertEquals('62696c6c696e672d64657461696c0a', EncodingUtil.convertToHex(Blob.valueOf(response.getBody())));
  }
}
