@IsTest private class SchemaIsolationPlainTest {
    @IsTest static void row0() { String observed = SchemaIsolationProbe.observe(); System.debug('SCHEMA_ROW:' + observed); System.assertEquals('0|Owned transient|1|Name', observed); }
    @IsTest static void row1() { String observed = SchemaIsolationProbe.observe(); System.debug('SCHEMA_ROW:' + observed); System.assertEquals('0|Owned transient|1|Name', observed); }
    @IsTest static void row2() { System.runAs(new User(Id = UserInfo.getUserId())) { String observed = SchemaIsolationProbe.observe(); System.debug('SCHEMA_ROW:' + observed); System.assertEquals('0|Owned transient|1|Name', observed); } }
    @IsTest static void row3() { System.runAs(new User(Id = UserInfo.getUserId())) { String observed = SchemaIsolationProbe.observe(); System.debug('SCHEMA_ROW:' + observed); System.assertEquals('0|Owned transient|1|Name', observed); } }
}
