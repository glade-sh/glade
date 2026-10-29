@IsTest private class PendingRowsObservationTest {
 @IsTest static void NV001(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';

 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV001|'+JSON.serialize(o));}
 @IsTest static void NV002(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV002|'+JSON.serialize(o));}
 @IsTest static void NV003(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV003|'+JSON.serialize(o));}
 @IsTest static void NV004(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV004|'+JSON.serialize(o));}
 @IsTest static void NV005(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV005|'+JSON.serialize(o));}
 @IsTest static void NV006(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle('');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV006|'+JSON.serialize(o));}
 @IsTest static void NV007(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle('t'.repeat(251));
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV007|'+JSON.serialize(o));}
 @IsTest static void NV008(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setBody(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV008|'+JSON.serialize(o));}
 @IsTest static void NV009(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setBody('');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV009|'+JSON.serialize(o));}
 @IsTest static void NV010(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setBody('b'.repeat(751));
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV010|'+JSON.serialize(o));}
 @IsTest static void NV011(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV011|'+JSON.serialize(o));}
 @IsTest static void NV012(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId('');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV012|'+JSON.serialize(o));}
 @IsTest static void NV013(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV013|'+JSON.serialize(o));}
 @IsTest static void NV014(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV014|'+JSON.serialize(o));}
 @IsTest static void NV015(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV015|'+JSON.serialize(o));}
 @IsTest static void NV016(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients.clear();for(Integer i=1;i<=501;i++)recipients.add('005'+String.valueOf(i).leftPad(12,'0'));
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV016|'+JSON.serialize(o));}
 @IsTest static void NV017(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=new Set<String>{null};
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV017|'+JSON.serialize(o));}
 @IsTest static void NV018(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String valid=String.valueOf(UserInfo.getUserId());recipients=new Set<String>{valid.substring(0,17)+(valid.endsWith('A')?'B':'A')};
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV018|'+JSON.serialize(o));}
 @IsTest static void NV019(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV019|'+JSON.serialize(o));}
 @IsTest static void NV020(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"standard__namedPage","attributes":{"pageName":"home"}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV020|'+JSON.serialize(o));}
 @IsTest static void NV021(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients.clear();for(Integer i=1;i<=500;i++)recipients.add('005'+String.valueOf(i).leftPad(12,'0'));
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV021|'+JSON.serialize(o));}
 @IsTest static void NV022(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle(null);n.setBody(null);n.setTargetId(null);recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV022|'+JSON.serialize(o));}
 @IsTest static void NV023(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle('t'.repeat(250));n.setBody('b'.repeat(750));
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV023|'+JSON.serialize(o));}
 @IsTest static void NV024(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('0ML000000000099');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV024|'+JSON.serialize(o));}
 @IsTest static void NV025(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('not-json');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|NV025|'+JSON.serialize(o));}
 @IsTest static void OP_1(){Object o=PermissionOmissionObservation.observe('OP_1');
System.assert(false,'OBS|OP_1|'+JSON.serialize(o));}
 @IsTest static void OP_2(){Object o=PermissionOmissionObservation.observe('OP_2');
System.assert(false,'OBS|OP_2|'+JSON.serialize(o));}
 @IsTest static void OP_3(){Object o=PermissionOmissionObservation.observe('OP_3');
System.assert(false,'OBS|OP_3|'+JSON.serialize(o));}
 @IsTest static void OP_R(){Object o=PermissionOmissionObservation.observe('OP_R');
System.assert(false,'OBS|OP_R|'+JSON.serialize(o));}
}
