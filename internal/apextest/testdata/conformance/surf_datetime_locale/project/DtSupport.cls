public class DtSupport {
 public static User mk(String locale, String tz, String alias, String lang) {
  User u = new User(
   LastName = 'Probe',
   Alias = alias,
   Username = 'dt' + alias + Datetime.now().getTime() + '@example.invalid',
   Email = 'dt@example.invalid',
   TimeZoneSidKey = tz,
   LocaleSidKey = locale,
   EmailEncodingKey = 'UTF-8',
   LanguageLocaleKey = lang,
   ProfileId = UserInfo.getProfileId()
  );
  insert u;
  return u;
 }
 public static List<User> quartet() {
  List<User> us = new List<User>();
  us.add(mk('en_US', 'America/Los_Angeles', 'usla', 'en_US'));
  us.add(mk('en_GB', 'Asia/Tokyo', 'gbtok', 'en_US'));
  us.add(mk('de_DE', 'Europe/London', 'delon', 'de'));
  us.add(mk('ja_JP', 'America/Los_Angeles', 'jpla', 'ja'));
  return us;
 }
 public static List<User> zones() {
  List<User> us = new List<User>();
  us.add(mk('en_US', 'America/Los_Angeles', 'zla', 'en_US'));
  us.add(mk('en_GB', 'Europe/London', 'zlon', 'en_US'));
  us.add(mk('ja_JP', 'Asia/Tokyo', 'ztok', 'ja'));
  return us;
 }
 public static String stamp(Datetime d) {
  return String.valueOf(d.getTime()) + '@' + d.format('yyyy-MM-dd HH:mm:ss z');
 }
 public static String tag(User u) {
  return u.LocaleSidKey.left(2) + '/' + u.TimeZoneSidKey.substringAfterLast('/').left(3);
 }
}
