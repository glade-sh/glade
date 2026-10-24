public class AuthContextProbe {
 public static String observe() {
  Map<String,String> rows = new Map<String,String>();
  rows.put('isTest', String.valueOf(Test.isRunningTest()));
  rows.put('siteId', String.valueOf(Site.getSiteId()));
  rows.put('networkId', String.valueOf(Network.getNetworkId()));
  try { rows.put('siteCount', String.valueOf(Database.countQuery('SELECT COUNT() FROM Site'))); } catch (Exception e) { rows.put('siteCount', 'UNAVAILABLE'); }
  try { rows.put('networkCount', String.valueOf(Database.countQuery('SELECT COUNT() FROM Network'))); } catch (Exception e) { rows.put('networkCount', 'UNAVAILABLE'); }
  User externalUser = new User(Username='visitor@example.invalid', Email='visitor@example.invalid', LastName='Visitor', Alias='visit');
  User portalUser = externalUser.clone();
  List<Profile> portalProfiles = [SELECT Id FROM Profile WHERE UserType IN ('CSPLitePortal','PowerCustomerSuccess','PowerPartner','CustomerSuccess') LIMIT 1];
  if (!portalProfiles.isEmpty()) portalUser.ProfileId = portalProfiles[0].Id;
  rows.put('portalProfile', String.valueOf(!portalProfiles.isEmpty()));
  try { rows.put('login_nonblank_null', describe(Site.login('visitor@example.invalid', 'invalid', null))); } catch (Exception e) { rows.put('login_nonblank_null', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('login_nonblank_path', describe(Site.login('visitor@example.invalid', 'invalid', '/home'))); } catch (Exception e) { rows.put('login_nonblank_path', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('login_blank_null', describe(Site.login('', '', null))); } catch (Exception e) { rows.put('login_blank_null', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('login_blank_path', describe(Site.login('', '', '/home'))); } catch (Exception e) { rows.put('login_blank_path', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('login_null_null', describe(Site.login(null, null, null))); } catch (Exception e) { rows.put('login_null_null', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('forward_one_null', describe(Network.forwardToAuthPage(null))); } catch (Exception e) { rows.put('forward_one_null', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('forward_one_path', describe(Network.forwardToAuthPage('/home'))); } catch (Exception e) { rows.put('forward_one_path', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('forward_two_null', describe(Network.forwardToAuthPage(null, null))); } catch (Exception e) { rows.put('forward_two_null', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('forward_two_display', describe(Network.forwardToAuthPage('/home', 'page'))); } catch (Exception e) { rows.put('forward_two_display', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('forward_two_path', describe(Network.forwardToAuthPage('/x', null))); } catch (Exception e) { rows.put('forward_two_path', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('external_empty', describe(Network.createExternalUserAsync(new User(), new Contact(), new Account()))); } catch (Exception e) { rows.put('external_empty', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  try { rows.put('external_null_profile', describe(Network.createExternalUserAsync(externalUser, new Contact(LastName='Visitor'), new Account(Name='Visitor account')))); } catch (Exception e) { rows.put('external_null_profile', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  if (portalProfiles.isEmpty()) rows.put('external_portal_profile', 'UNAVAILABLE'); else {
  try { rows.put('external_portal_profile', describe(Network.createExternalUserAsync(portalUser, new Contact(LastName='Visitor'), new Account(Name='Visitor account')))); } catch (Exception e) { rows.put('external_portal_profile', 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage()); }
  }
  return JSON.serialize(rows);
 }
 private static String describe(Object result) {
  if (result == null) return 'NULL';
  if (result instanceof PageReference) return 'PAGE|' + ((PageReference)result).getUrl();
  return 'VALUE|' + String.valueOf(result);
 }
}
