// Anonymous probe: prove anonymous context agrees with @IsTest.
Datetime a = Datetime.newInstanceGmt(Date.newInstance(1970, 1, 1), Time.newInstance(0, 0, 0, 0));
Datetime b = Datetime.valueOfGmt('1969-12-31 23:59:59.999');
List<Datetime> l = new List<Datetime>{ a, b };
Set<Datetime> s = new Set<Datetime>{ a, b };
String p = 'anon';
p += '|A=' + a.hashCode() + ';' + a.getTime() + ';' + System.hashCode(a);
p += '|B=' + b.hashCode() + ';' + b.getTime() + ';' + System.hashCode(b);
p += '|L=' + l.hashCode() + ';' + l.size();
p += '|S=' + s.hashCode() + ';' + s.size();
p += '|T=' + Time.newInstance(12, 34, 56, 789).hashCode();
p += '|D=' + Date.newInstance(2026, 11, 16).hashCode();
throw new System.AssertException(p);
