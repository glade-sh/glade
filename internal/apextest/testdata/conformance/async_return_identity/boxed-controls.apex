Id shortId = '707000000000001';
Id longId = '707000000000001AAA';
Object boxedShort = shortId;
Object boxedLong = longId;
Object stringShort = '707000000000001';
Object stringLong = '707000000000001AAA';
Map<String,Object> observation = new Map<String,Object>{
    'typedShortLength' => String.valueOf(shortId).length(),
    'typedLongLength' => String.valueOf(longId).length(),
    'boxedShortLength' => String.valueOf(boxedShort).length(),
    'boxedLongLength' => String.valueOf(boxedLong).length(),
    'castShortLength' => String.valueOf((Id) boxedShort).length(),
    'castLongLength' => String.valueOf((Id) boxedLong).length(),
    'stringShortLength' => String.valueOf(stringShort).length(),
    'stringLongLength' => String.valueOf(stringLong).length(),
    'nullValue' => String.valueOf((Object) null)
};
System.debug(LoggingLevel.ERROR, 'OBS|' + JSON.serialize(observation));
