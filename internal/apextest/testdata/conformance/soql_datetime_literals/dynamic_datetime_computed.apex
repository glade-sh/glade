Object r;
Datetime boundary = Datetime.newInstanceGmt(2020, 1, 1, 8, 0, 0);
String subject = 'Synthetic computed datetime';
delete [SELECT Id FROM Event WHERE Subject = :subject];
insert new List<Event>{
    new Event(Subject = subject, StartDateTime = boundary.addMinutes(-1), EndDateTime = boundary.addHours(1)),
    new Event(Subject = subject, StartDateTime = boundary, EndDateTime = boundary.addHours(1)),
    new Event(Subject = subject, StartDateTime = boundary.addMinutes(1), EndDateTime = boundary.addHours(1))
};
String literal = boundary.formatGmt('yyyy-MM-dd') + 'T' + boundary.formatGmt('HH:mm:ss') + 'Z';
String query = 'SELECT Id FROM Event WHERE Subject = \'Synthetic computed datetime\' AND StartDateTime > ' + literal;
List<Event> afterRows = Database.query(query);
r = String.valueOf(afterRows.size());
System.debug('SOQL_DATETIME|dynamic_datetime_computed|' + String.valueOf(r));
