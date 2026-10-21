Set<String> values = new Set<String>{'alpha', 'beta'};
Iterator<String> cursor = values.iterator();
String selected = cursor.next();
values.remove(selected);
cursor.next();
