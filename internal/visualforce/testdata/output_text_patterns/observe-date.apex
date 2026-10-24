System.debug('LOCALE|' + UserInfo.getLocale());
String body = new PageReference('/apex/PatternProbe29').getContent().toString();
Matcher m = Pattern.compile('(?s)<div data-case="([^"]+)">(.*?)</div>').matcher(body);
while (m.find()) { System.debug('PATTERNROW|PatternProbe29|' + m.group(1) + '|' + EncodingUtil.base64Encode(Blob.valueOf(m.group(2)))); }
