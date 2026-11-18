@IsTest private class OverrideCheck { @IsTest static void check() { Integer result = OverrideProbe.run(); System.assertNotEquals(null, result); } }
