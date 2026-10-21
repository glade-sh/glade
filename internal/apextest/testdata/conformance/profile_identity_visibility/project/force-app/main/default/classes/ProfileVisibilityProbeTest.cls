@IsTest public class ProfileVisibilityProbeTest {
 @IsTest static void observe() {
  System.assert(false, 'OBS|' + ProfileVisibilityProbe.sample());
 }
}
