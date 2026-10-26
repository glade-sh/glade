@IsTest private class EmailActivityTest {
 @IsTest static void success(){System.assert(false,'OBSERVED|success|'+JSON.serialize(EmailActivityProbe.observe('success')));}
 @IsTest static void disabled(){System.assert(false,'OBSERVED|disabled|'+JSON.serialize(EmailActivityProbe.observe('disabled')));}
 @IsTest static void missingWhat(){System.assert(false,'OBSERVED|missingWhat|'+JSON.serialize(EmailActivityProbe.observe('missingWhat')));}
 @IsTest static void userWhat(){System.assert(false,'OBSERVED|userWhat|'+JSON.serialize(EmailActivityProbe.observe('userWhat')));}
 @IsTest static void disabledUserWhat(){System.assert(false,'OBSERVED|disabledUserWhat|'+JSON.serialize(EmailActivityProbe.observe('disabledUserWhat')));}
 @IsTest static void contactWhat(){System.assert(false,'OBSERVED|contactWhat|'+JSON.serialize(EmailActivityProbe.observe('contactWhat')));}
 @IsTest static void missingReference(){System.assert(false,'OBSERVED|missingReference|'+JSON.serialize(EmailActivityProbe.observe('missingReference')));}
 @IsTest static void deletedWhat(){System.assert(false,'OBSERVED|deletedWhat|'+JSON.serialize(EmailActivityProbe.observe('deletedWhat')));}
 @IsTest static void addressOnly(){System.assert(false,'OBSERVED|addressOnly|'+JSON.serialize(EmailActivityProbe.observe('addressOnly')));}
 @IsTest static void detachedTarget(){System.assert(false,'OBSERVED|detachedTarget|'+JSON.serialize(EmailActivityProbe.observe('detachedTarget')));}
 @IsTest static void bulkSend(){System.assert(false,'OBSERVED|bulkSend|'+JSON.serialize(EmailActivityProbe.observe('bulkSend')));}
 @IsTest static void partial(){System.assert(false,'OBSERVED|partial|'+JSON.serialize(EmailActivityProbe.observe('partial')));}
 @IsTest static void allOrNone(){System.assert(false,'OBSERVED|allOrNone|'+JSON.serialize(EmailActivityProbe.observe('allOrNone')));}
 @IsTest static void rollback(){System.assert(false,'OBSERVED|rollback|'+JSON.serialize(EmailActivityProbe.observe('rollback')));}
 @IsTest static void template(){System.assert(false,'OBSERVED|template|'+JSON.serialize(EmailActivityProbe.observe('template')));}
}
