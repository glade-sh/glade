public class CallbackShapeProbe {
 public static Map<String,String> observe() {
  Map<String,String> results = new Map<String,String>();
  for (Integer shape=0; shape<3; shape++) {
   try {
    CallbackShapeProvider provider = new CallbackShapeProvider();
    CallbackShapeService service = (CallbackShapeService)Test.createStub(CallbackShapeService.class,provider);
    if (shape==0) service.accept(new List<Account>{new Account(Name='shape')});
    if (shape==1) service.accept(new List<Account>());
    if (shape==2) service.accept(new List<SObject>{new Account(Name='shape')});
    results.put(String.valueOf(shape),provider.observed);
   } catch (Exception problem) { results.put(String.valueOf(shape),problem.getTypeName()+'|'+problem.getMessage()); }
  }
  return results;
 }
}
