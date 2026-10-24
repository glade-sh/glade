@IsTest private class AsyncAccessProbeTest {
 @IsTest static void chainRejected() { System.assert(false,'OBSERVED|chainRejected|'+JSON.serialize(AsyncAccessProbe.observe('chainRejected'))); }
 @IsTest static void unchained() { System.assert(false,'OBSERVED|unchained|'+JSON.serialize(AsyncAccessProbe.observe('unchained'))); }
 @IsTest static void terminalUpdate() { System.assert(false,'OBSERVED|terminalUpdate|'+JSON.serialize(AsyncAccessProbe.observe('terminalUpdate'))); }
 @IsTest static void accessUser() { System.assert(false,'OBSERVED|accessUser|'+JSON.serialize(AsyncAccessProbe.observe('accessUser'))); }
 @IsTest static void accessSystem() { System.assert(false,'OBSERVED|accessSystem|'+JSON.serialize(AsyncAccessProbe.observe('accessSystem'))); }
 @IsTest static void accessCustom() { System.assert(false,'OBSERVED|accessCustom|'+JSON.serialize(AsyncAccessProbe.observe('accessCustom'))); }
}
