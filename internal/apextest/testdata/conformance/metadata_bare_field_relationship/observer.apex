List<String> shapes = new List<String>{'bare-column','bare-standard-column','bare-entity','bare-column-durable','bare-groups','static-init'};
for (String s : shapes) {
 try { System.debug('ROW|' + s + '|' + MeadowBareProbe.observe(s)); }
 catch (Exception e) { System.debug('ROW|' + s + '|EXC:' + e.getTypeName() + ':' + e.getMessage()); }
}
