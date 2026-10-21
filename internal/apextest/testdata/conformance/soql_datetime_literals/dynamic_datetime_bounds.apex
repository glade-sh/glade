Object r;
Datetime boundary = Datetime.newInstanceGmt(2020, 1, 1, 8, 0, 0);
String subject = 'Synthetic dynamic datetime';
delete [SELECT Id FROM Event WHERE Subject = :subject];
insert new List<Event>{
    new Event(Subject = subject, StartDateTime = boundary.addMinutes(-1), EndDateTime = boundary.addHours(1)),
    new Event(Subject = subject, StartDateTime = boundary, EndDateTime = boundary.addHours(1)),
    new Event(Subject = subject, StartDateTime = boundary.addMinutes(1), EndDateTime = boundary.addHours(1))
};
List<Event> afterRows = Database.query('SELECT Id FROM Event WHERE Subject = \'Synthetic dynamic datetime\' AND StartDateTime > 2020-01-01T08:00:00Z');
List<Event> beforeRows = Database.query('SELECT Id FROM Event WHERE Subject = \'Synthetic dynamic datetime\' AND StartDateTime < 2020-01-01T08:00:00Z');
r = String.valueOf(afterRows.size()) + '|' + String.valueOf(beforeRows.size());
System.debug('SOQL_DATETIME|dynamic_datetime_bounds|' + String.valueOf(r));
