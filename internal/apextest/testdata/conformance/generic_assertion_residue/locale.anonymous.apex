System.debug(LoggingLevel.ERROR, 'TRACE|localeDateFormat|before');
Date sample = Date.newInstance(2025, 11, 23);
String formattedDate = sample.format();
System.debug(LoggingLevel.ERROR, 'TRACE|localeDatetimeFormat|before');
Datetime stamp = Datetime.newInstance(2025, 11, 23, 14, 30, 0);
System.debug(LoggingLevel.ERROR, 'OBS|localeDateFormat|' + JSON.serialize(new Map<String, Object>{
    'locale' => UserInfo.getLocale(),
    'dateFormat' => formattedDate,
    'datetimeFormat' => stamp.format()
}));
