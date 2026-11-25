String pattern = 'E|EE|EEE|EEEE|EEEEE|MMM|MMMM|MMMMM|LLL|LLLL|LLLLL|u';
List<Datetime> dates = new List<Datetime>();
for (Integer month = 1; month <= 12; month++) {
    dates.add(Datetime.newInstanceGmt(1500, month, 15, 12, 0, 0));
}
for (Integer day = 1; day <= 7; day++) {
    dates.add(Datetime.newInstanceGmt(1400, 1, day, 12, 0, 0));
}
dates.add(Datetime.newInstanceGmt(1500, 2, 29, 12, 0, 0));
dates.add(Datetime.newInstanceGmt(1582, 10, 4, 12, 0, 0));
dates.add(Datetime.newInstanceGmt(1582, 10, 15, 12, 0, 0));
dates.add(Datetime.newInstanceGmt(1582, 12, 31, 12, 0, 0));
dates.add(Datetime.newInstanceGmt(1583, 1, 1, 12, 0, 0));
System.debug(LoggingLevel.ERROR, 'RD473_LOCALE=' + UserInfo.getLocale() + '|' + UserInfo.getLanguage() + '|' + UserInfo.getTimeZone().getID());
for (Integer i = 0; i < dates.size(); i++) {
    Datetime d = dates[i];
    System.debug(LoggingLevel.ERROR, 'RD473_ROW=' + i + '|local|' + d.format(pattern));
    System.debug(LoggingLevel.ERROR, 'RD473_ROW=' + i + '|explicit|' + d.format(pattern, 'GMT'));
    System.debug(LoggingLevel.ERROR, 'RD473_ROW=' + i + '|gmt|' + d.formatGmt(pattern));
}
