LongtailAsyncSoapService service = new LongtailAsyncSoapService();
System.Continuation continuation = new System.Continuation(60);
Integer before = Limits.getCallouts();
String observation = 'returned';
try { service.beginEcho(continuation, 'anonymous'); }
catch (Exception problem) { observation = problem.getTypeName() + '|' + problem.getMessage(); }
System.assertEquals('System.CalloutException|Web service callout failed: Unable to find namespace-typemap info. Please regenerate the stub', observation);
System.assertEquals(1, Limits.getCallouts() - before);
