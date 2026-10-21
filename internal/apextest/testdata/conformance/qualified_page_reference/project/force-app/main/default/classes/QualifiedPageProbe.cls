public class QualifiedPageProbe {
 public static Map<String,Object> observe() {
  PageReference qualified = System.Page.Meadow;
  PageReference unqualified = Page.Meadow;
  qualified.setRedirect(true);
  qualified.getParameters().put('color', 'green');
  Map<String,Object> result = new Map<String,Object>();
  result.put('qualifiedUrl', qualified.getUrl());
  result.put('unqualifiedUrl', unqualified.getUrl());
  result.put('redirect', qualified.getRedirect());
  result.put('color', qualified.getParameters().get('color'));
  return result;
 }
}
