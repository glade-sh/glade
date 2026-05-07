@isTest
private class ApexMocksProxyTest {
  @isTest static void proxyKeepsProviderLifecycleAndStubIdentity() {
    ProxyRecorder recorder = new ProxyRecorder();
    ProxyList proxy = Test.createStub(ProxyList.class, recorder);
    recorder.expectedStub = proxy;

    System.assertEquals('hello', proxy.get2(0, 'Hi Hello'));
    System.assertEquals(1, recorder.calls);
    System.assertEquals('get2', recorder.lastMethod);
    System.assertEquals('Hi Hello', recorder.lastStringArg);
    System.assertEquals(true, recorder.sawSameStub);

    proxy.add('recorded');
    System.assertEquals(2, recorder.calls);
    System.assertEquals('add', recorder.lastMethod);
    System.assertEquals('recorded', recorder.lastStringArg);
    System.assertEquals(true, recorder.sawSameStub);
  }
}
