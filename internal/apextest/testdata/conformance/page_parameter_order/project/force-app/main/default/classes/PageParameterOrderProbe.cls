public class PageParameterOrderProbe {
 public static String sample() {
  List<String> keys = new List<String>{'rowId', 'rowaId', 'id', 'Zulu', 'alpha'};
  PageReference forward = new PageReference('/apex/MeadowView');
  PageReference reverse = new PageReference('/apex/MeadowView');
  for (String key : keys) forward.getParameters().put(key, key);
  for (Integer i=keys.size()-1; i>=0; i--) reverse.getParameters().put(keys[i], keys[i]);
  PageReference encoded = new PageReference('/apex/MeadowView');
  encoded.getParameters().put('value', 'blue sky/*');
  PageReference raw = new PageReference('/apex/MeadowView?rowId=rowId&rowaId=rowaId&id=id&Zulu=Zulu&alpha=alpha');
  return JSON.serialize(new Map<String,Object>{
   'forward' => forward.getUrl(),
   'reverse' => reverse.getUrl(),
   'encoded' => encoded.getUrl(),
   'rawConstructor' => raw.getUrl()
  });
 }
}
