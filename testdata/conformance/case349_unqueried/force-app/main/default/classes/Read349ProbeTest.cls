@IsTest private class Read349ProbeTest {
  @IsTest static void exercise() {
    List<Object> rows=Read349Probe.run(); System.debug('CASE349_READ='+JSON.serialize(rows)); System.assertEquals(3,rows.size());
    for (Object row : rows) { Map<String,Object> r=(Map<String,Object>)row; System.assertEquals(true,r.get('unqueriedRejected')); }
  }
}
