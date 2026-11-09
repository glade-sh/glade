trigger TWAcctTrg on Account (before insert, after insert) {
  for (Account a : Trigger.new) {
    String n = a.Name;
    if (Trigger.isBefore) {
      if (n == 'b-assert') System.assert(false, 'before assert msg');
      if (n == 'b-eq') System.assertEquals(1, 2, 'before eq msg');
      if (n == 'b-nomsg') System.assert(false);
      if (n == 'b-ne') System.assertNotEquals(1, 1, 'before ne msg');
      if (n == 'b-assertapi') Assert.isTrue(false, 'before Assert msg');
      if (n == 'b-adderr') a.addError('before adderr msg');
      if (n == 'b-throw') throw new TWException('before throw msg');
      if (n == 'b-api-nomsg') Assert.isTrue(false);
      if (n == 'b-api-false') Assert.isFalse(true, 'api false msg');
      if (n == 'b-api-eq') Assert.areEqual(1, 2, 'api eq msg');
      if (n == 'b-api-ne') Assert.areNotEqual(1, 1, 'api ne msg');
      if (n == 'b-api-null') Assert.isNull(1, 'api null msg');
      if (n == 'b-api-notnull') Assert.isNotNull(null, 'api notnull msg');
      if (n == 'b-api-fail') Assert.fail('api fail msg');
      if (n == 'b-api-inst') Assert.isInstanceOfType('x', Integer.class, 'api inst msg');
      if (n == 'b-cls-sys') TWDeep.sys();
      if (n == 'b-cls-api') TWDeep.api();
      if (n == 'b-cls-deep') TWDeep.outer();
      if (n == 'b-cls-throw') TWDeep.thrower();
      if (n == 'b-swallow') { try { System.assert(false, 'swallowed'); a.Description = 'not caught'; } catch (Exception e) { a.Description = 'caught ' + e.getTypeName(); } }
    } else {
      if (n == 'a-assert') System.assert(false, 'after assert msg');
      if (n == 'a-eq') System.assertEquals(1, 2, 'after eq msg');
      if (n == 'a-nomsg') System.assert(false);
      if (n == 'a-api') Assert.isTrue(false, 'after api msg');
      if (n == 'a-cls-deep') TWDeep.outer();
      if (n == 'a-adderr') a.addError('after adderr msg');
    }
  }
}