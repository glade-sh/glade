@IsTest private class ProtectedCheck { @IsTest static void check() { Integer result = ProtectedProbe.run(); System.assertNotEquals(null, result); } }
