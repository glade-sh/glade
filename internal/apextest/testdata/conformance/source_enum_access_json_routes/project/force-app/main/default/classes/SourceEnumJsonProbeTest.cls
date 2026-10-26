@IsTest private class SourceEnumJsonProbeTest {
 @IsTest static void shadow(){System.assert(false,'OBSERVED|shadow|'+JSON.serialize(SourceEnumJsonProbe.observe('shadow')));}
 @IsTest static void ordinary(){System.assert(false,'OBSERVED|ordinary|'+JSON.serialize(SourceEnumJsonProbe.observe('ordinary')));}
}
