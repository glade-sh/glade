@IsTest public class ReflectionObservationTest {
 private class PrivateChild extends ReflectionProbeBase {}
 @IsTest static void capture() {
  Map<String,Object> rows=ReflectionObservation.observe();
  rows.put('privateTest',ReflectionObservation.inspect(new PrivateChild(),'ReflectionObservationTest.PrivateChild'));
  System.assert(false,'NATIVE_REFLECTION:'+JSON.serialize(rows));
 }
}
