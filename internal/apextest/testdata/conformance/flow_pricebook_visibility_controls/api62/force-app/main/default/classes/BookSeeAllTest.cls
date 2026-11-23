@IsTest(SeeAllData=true) private class BookSeeAllTest { @IsTest static void probe() { System.assert(false, 'OBS=' + LookupProbeCase.book()); } }
