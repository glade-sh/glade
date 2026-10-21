public class PageAttributeProbe {
 public static String sample() {
  return JSON.serialize(new Map<String,Object>{'url' => Page.MeadowAttributePage.getUrl()});
 }
}
