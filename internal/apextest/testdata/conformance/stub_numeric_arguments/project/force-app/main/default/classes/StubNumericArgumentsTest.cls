@IsTest private class StubNumericArgumentsTest {
 @IsTest static void overloaded() {
  AmountRecorder recorder = new AmountRecorder();
  AmountService service = (AmountService)Test.createStub(AmountService.class, recorder);
  String answer = AmountFacade.invoke(service);
  System.assert(false, 'OBSERVED|overloaded|' + JSON.serialize(new Map<String,Object>{
   'answer'=>answer, 'calls'=>recorder.calls,
   'numericListControl'=>new List<Object>{7} == new List<Object>{Decimal.valueOf('7')},
   'realControl'=>new AmountService().format(7)}));
 }
 @IsTest static void scalar_controls() {
  AmountRecorder recorder = new AmountRecorder();
  AmountService service = (AmountService)Test.createStub(AmountService.class, recorder);
  String first = service.scalar(7, 7);
  Object second = service.erased(7);
  System.assert(false, 'OBSERVED|scalar_controls|' + JSON.serialize(new Map<String,Object>{
   'first'=>first, 'second'=>second, 'calls'=>recorder.calls}));
 }
}
