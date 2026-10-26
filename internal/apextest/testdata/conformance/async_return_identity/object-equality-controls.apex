String shortText = '707000000000001';
String longText = '707000000000001AAA';
Object shortObject = shortText;
Object longObject = longText;
Object ordinaryShort = 'abcdefghijklmno';
Object ordinaryLong = 'abcdefghijklmnoAAA';
Map<String,Object> observation = new Map<String,Object>{
    'objectJobEquals' => shortObject.equals(longObject),
    'objectJobReverseEquals' => longObject.equals(shortObject),
    'stringJobEquals' => shortText.equals(longText),
    'objectOrdinaryEquals' => ordinaryShort.equals(ordinaryLong),
    'objectMismatchEquals' => shortObject.equals('707000000000002AAA'),
    'objectSameLengthEquals' => ordinaryShort.equals('ABCDEFGHIJKLMNO')
};
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(observation));
