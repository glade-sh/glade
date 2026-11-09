System.debug('CASE366|control|' + JSON.serialize('0.0.0.0'.split('\\.', -1)));
System.debug('CASE366|boundary|' + JSON.serialize('0.0.0.0'.split('\\.', 1)));
try {
    Pattern.compile('[');
    System.debug('CASE366|rejection|accepted');
} catch (Exception e) {
    System.debug('CASE366|rejection|' + e.getTypeName());
}
