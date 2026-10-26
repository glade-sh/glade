@IsTest private class AccessJsonProbeTest {
 @IsTest static void jsonUser(){System.assert(false,'OBSERVED|jsonUser|'+JSON.serialize(AccessJsonProbe.observe('jsonUser')));}
 @IsTest static void jsonSystem(){System.assert(false,'OBSERVED|jsonSystem|'+JSON.serialize(AccessJsonProbe.observe('jsonSystem')));}
 @IsTest static void identityUser(){System.assert(false,'OBSERVED|identityUser|'+JSON.serialize(AccessJsonProbe.observe('identityUser')));}
 @IsTest static void identitySystem(){System.assert(false,'OBSERVED|identitySystem|'+JSON.serialize(AccessJsonProbe.observe('identitySystem')));}
}
