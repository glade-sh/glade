public static String pub() { return 'p'; }
private String priv() { return 'q'; }
System.debug('CAP|lm|' + pub() + priv());
