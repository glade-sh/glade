@IsTest public class PageAttributeProbeTest {
 @IsTest static void observe() {
  System.assert(false, 'OBS|' + PageAttributeProbe.sample());
 }
}
