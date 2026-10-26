Savepoint beforeProbe=Database.setSavepoint();
try { System.assert(false,'TEMPORAL|ENTRY|'+JSON.serialize(TemporalQueryProbe.run('ENTRY'))); } finally { Database.rollback(beforeProbe); }
