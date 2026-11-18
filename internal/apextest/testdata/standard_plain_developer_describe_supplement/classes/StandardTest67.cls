@IsTest private class StandardTest67 {
 @IsTest static void tier(){System.assertEquals('EXPECTED',StandardProbe67.tier());}
 @IsTest static void accountType(){System.assertEquals('EXPECTED',StandardProbe67.pick('Account','Type'));}
 @IsTest static void caseOrigin(){System.assertEquals('EXPECTED',StandardProbe67.pick('Case','Origin'));}
 @IsTest static void caseStatus(){System.assertEquals('EXPECTED',StandardProbe67.pick('Case','Status'));}
 @IsTest static void contactSource(){System.assertEquals('EXPECTED',StandardProbe67.pick('Contact','LeadSource'));}
 @IsTest static void leadStatus(){System.assertEquals('EXPECTED',StandardProbe67.pick('Lead','Status'));}
 @IsTest static void oppStage(){System.assertEquals('EXPECTED',StandardProbe67.pick('Opportunity','StageName'));}
}
