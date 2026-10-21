public class AssetPageProbe {
 public static String sample() {
  return JSON.serialize(new Map<String,Object>{
   'assetPage' => Page.MeadowAssetPage.getUrl(),
   'plainPage' => Page.MeadowPlainPage.getUrl()
  });
 }
}
