for (Integer i = 0; i < 29; i++) {
 String pageName = 'PatternProbe' + i;
 String body = new PageReference('/apex/' + pageName).getContent().toString();
 Matcher m = Pattern.compile('(?s)<div data-case="([^"]+)">(.*?)</div>').matcher(body);
 while (m.find()) { System.debug('PATTERNROW|' + pageName + '|' + m.group(1) + '|' + EncodingUtil.base64Encode(Blob.valueOf(m.group(2)))); }
}
