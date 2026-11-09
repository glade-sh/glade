public class ApprovalSetupCapture {
 public static String observe() {
  String result=''; String suffix=String.valueOf(System.now().getTime());
  try { insert new PermissionSet(Name='SetupGrant'+suffix,Label='Setup Grant '+suffix); insert new MutingPermissionSet(DeveloperName='SetupMute'+suffix,MasterLabel='Setup Mute '+suffix); result+='setup=OK;'; }
  catch(DmlException e) { result+='setup='+String.valueOf(e.getDmlStatusCode(0))+';'; }
  try { insert new Account(Name='Mixed probe'); result+='mixed=OK;'; }
  catch(DmlException e) { result+='mixed='+String.valueOf(e.getDmlStatusCode(0))+';'; }
  return result;
 }
}
