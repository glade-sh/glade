public class InstallMemberProbe implements InstallHandler {
 public static Map<String,Object> observed;
 public void onInstall(InstallContext context) {
  observed=new Map<String,Object>();
  observed.put('organization',context.ORGANIZATIONID()==UserInfo.getOrganizationId());
  observed.put('installer',context.installerID()==UserInfo.getUserId());
  Version previous=context.PREVIOUSVERSION();
  observed.put('previous',previous==null ? null : previous.toString());
  observed.put('upgrade',context.ISUPGRADE());
  observed.put('push',context.ISPUSH());
 }
}
