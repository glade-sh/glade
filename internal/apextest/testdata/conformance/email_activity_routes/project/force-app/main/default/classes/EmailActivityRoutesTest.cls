@IsTest private class EmailActivityRoutesTest {
 @IsTest static void enabledDefault(){System.assert(false,'OBSERVED|enabledDefault|'+JSON.serialize(EmailActivityRoutesProbe.observe('enabledDefault')));}
 @IsTest static void enabledPartial(){System.assert(false,'OBSERVED|enabledPartial|'+JSON.serialize(EmailActivityRoutesProbe.observe('enabledPartial')));}
 @IsTest static void noActivitiesDefault(){System.assert(false,'OBSERVED|noActivitiesDefault|'+JSON.serialize(EmailActivityRoutesProbe.observe('noActivitiesDefault')));}
 @IsTest static void noActivitiesPartial(){System.assert(false,'OBSERVED|noActivitiesPartial|'+JSON.serialize(EmailActivityRoutesProbe.observe('noActivitiesPartial')));}
 @IsTest static void deletedDefault(){System.assert(false,'OBSERVED|deletedDefault|'+JSON.serialize(EmailActivityRoutesProbe.observe('deletedDefault')));}
 @IsTest static void deletedPartial(){System.assert(false,'OBSERVED|deletedPartial|'+JSON.serialize(EmailActivityRoutesProbe.observe('deletedPartial')));}
 @IsTest static void missingDefault(){System.assert(false,'OBSERVED|missingDefault|'+JSON.serialize(EmailActivityRoutesProbe.observe('missingDefault')));}
 @IsTest static void missingPartial(){System.assert(false,'OBSERVED|missingPartial|'+JSON.serialize(EmailActivityRoutesProbe.observe('missingPartial')));}
 @IsTest static void disabledDefault(){System.assert(false,'OBSERVED|disabledDefault|'+JSON.serialize(EmailActivityRoutesProbe.observe('disabledDefault')));}
 @IsTest static void disabledPartial(){System.assert(false,'OBSERVED|disabledPartial|'+JSON.serialize(EmailActivityRoutesProbe.observe('disabledPartial')));}
 @IsTest static void rollbackDefault(){System.assert(false,'OBSERVED|rollbackDefault|'+JSON.serialize(EmailActivityRoutesProbe.observe('rollbackDefault')));}
 @IsTest static void rollbackPartial(){System.assert(false,'OBSERVED|rollbackPartial|'+JSON.serialize(EmailActivityRoutesProbe.observe('rollbackPartial')));}
 @IsTest static void flowDirectTrue(){System.assert(false,'OBSERVED|flowDirectTrue|'+JSON.serialize(EmailActivityRoutesProbe.observe('flowDirectTrue')));}
 @IsTest static void flowDirectFalse(){System.assert(false,'OBSERVED|flowDirectFalse|'+JSON.serialize(EmailActivityRoutesProbe.observe('flowDirectFalse')));}
 @IsTest static void flowDirectOmitted(){System.assert(false,'OBSERVED|flowDirectOmitted|'+JSON.serialize(EmailActivityRoutesProbe.observe('flowDirectOmitted')));}
 @IsTest static void flowTriggerSubflowTrue(){System.assert(false,'OBSERVED|flowTriggerSubflowTrue|'+JSON.serialize(EmailActivityRoutesProbe.observe('flowTriggerSubflowTrue')));}
 @IsTest static void flowTriggerSubflowFalse(){System.assert(false,'OBSERVED|flowTriggerSubflowFalse|'+JSON.serialize(EmailActivityRoutesProbe.observe('flowTriggerSubflowFalse')));}
 @IsTest static void flowTriggerSubflowOmitted(){System.assert(false,'OBSERVED|flowTriggerSubflowOmitted|'+JSON.serialize(EmailActivityRoutesProbe.observe('flowTriggerSubflowOmitted')));}
}
