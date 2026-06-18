function stylesheet(token, useActualHostSelector, useNativeDirPseudoclass) {
  var shadowSelector = token ? ("[" + token + "]") : "";
  var hostSelector = token ? ("[" + token + "-host]") : "";
  var suffixToken = token ? ("-" + token) : "";
  return ((useActualHostSelector ? ":host {" : hostSelector + " {")) + "display: block;}div.slds-breadcrumb" + shadowSelector + " {list-style: none;}";
  /*LWC compiler v8.20.4*/
}
export default [stylesheet];