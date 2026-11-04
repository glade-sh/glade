@IsTest private class Review349ProbeTest {
    @IsTest static void exercise() {
        List<Object> rows = Review349Probe.run();
        System.debug('CASE349_REVIEW=' + JSON.serialize(rows));
        System.assertEquals(7, rows.size());
        Map<String,Object> a = (Map<String,Object>)rows[0];
        Map<String,Object> b = (Map<String,Object>)rows[1];
        Map<String,Object> c = (Map<String,Object>)rows[2];
        System.assertEquals(false, a.get('created')); System.assertEquals(false, b.get('created')); System.assertEquals(true, c.get('created'));
        for (Integer i=3; i<7; i++) { Map<String,Object> x=(Map<String,Object>)rows[i]; System.assertEquals(false, x.get('personType')); System.assertEquals(new List<Boolean>{false}, x.get('beforeKinds')); if (i == 3 || i == 5) { System.assertEquals(false,x.get('success')); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',x.get('status')); } else System.assertEquals(true,x.get('success')); }
    }
}
