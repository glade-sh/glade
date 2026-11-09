Reports.InvalidFilterException a = new Reports.InvalidFilterException('Synthetic invalid');
Reports.MetadataException b = null;
Reports.ReportRunException c = null;
if (b != null) System.debug(b.getMessage());
if (c != null) System.debug(c.getMessage());
System.debug('B6TYPED|' + a.getMessage() + '|' + a.getTypeName());
