for (String value : new List<String>{'false','x','x@y','synthetic@example.invalid','   '}) {
 System.debug('R349_EMAIL|' + (value=='   '?'spaces':value) + '|' + EmailBoundaryProbe.capture(value));
}
