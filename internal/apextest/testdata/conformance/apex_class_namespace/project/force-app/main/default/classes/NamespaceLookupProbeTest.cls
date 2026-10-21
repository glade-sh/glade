@IsTest public class NamespaceLookupProbeTest {
 @IsTest static void observe() {
  System.assert(false, 'OBS|' + NamespaceLookupProbe.sample());
 }
}
