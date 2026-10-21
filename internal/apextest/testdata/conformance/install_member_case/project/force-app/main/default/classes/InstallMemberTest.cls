@IsTest private class InstallMemberTest {
 @IsTest static void fresh_install() {
  Test.testInstall(new InstallMemberProbe(),null);
  System.assert(false,'OBSERVED|fresh_install|'+JSON.serialize(InstallMemberProbe.observed));
 }
 @IsTest static void push_upgrade() {
  Test.testInstall(new InstallMemberProbe(),new Version(1,2,3),true);
  System.assert(false,'OBSERVED|push_upgrade|'+JSON.serialize(InstallMemberProbe.observed));
 }
}
