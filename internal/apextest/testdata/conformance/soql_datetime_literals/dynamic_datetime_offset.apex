Object r;
Datetime boundary = Datetime.newInstanceGmt(2020, 1, 1, 8, 0, 0);
String subject = 'Synthetic datetime offset';
delete [SELECT Id FROM Event WHERE Subject = :subject];
insert new List<Event>{
    new Event(Subject = subject, StartDateTime = boundary.addMinutes(-1), EndDateTime = boundary.addHours(1)),
    new Event(Subject = subject, StartDateTime = boundary, EndDateTime = boundary.addHours(1)),
    new Event(Subject = subject, StartDateTime = boundary.addMinutes(1), EndDateTime = boundary.addHours(1))
};
List<Event> afterRows = Database.query('SELECT Id FROM Event WHERE Subject = \'Synthetic datetime offset\' AND StartDateTime > 2020-01-01T09:00:00+01:00');
r = String.valueOf(afterRows.size());
System.debug('SOQL_DATETIME|dynamic_datetime_offset|' + String.valueOf(r));
