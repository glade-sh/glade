@IsTest private class Case289ObservationTest {
 @IsTest static void PR01(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('bad-id');n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR01|'+JSON.serialize(o));}
 @IsTest static void PR02(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('bad-id');n.setTargetId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR02|'+JSON.serialize(o));}
 @IsTest static void PR03(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('bad-id');recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR03|'+JSON.serialize(o));}
 @IsTest static void PR04(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId(null);n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR04|'+JSON.serialize(o));}
 @IsTest static void PR05(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId(null);n.setTargetId(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR05|'+JSON.serialize(o));}
 @IsTest static void PR06(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId(null);recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR06|'+JSON.serialize(o));}
 @IsTest static void PR07(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle(null);n.setTargetId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR07|'+JSON.serialize(o));}
 @IsTest static void PR08(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle(null);recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR08|'+JSON.serialize(o));}
 @IsTest static void PR09(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId('bad-id');recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR09|'+JSON.serialize(o));}
 @IsTest static void PR10(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR10|'+JSON.serialize(o));}
 @IsTest static void PR11(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setBody(null);n.setTargetId(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR11|'+JSON.serialize(o));}
 @IsTest static void PR12(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setBody(null);recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR12|'+JSON.serialize(o));}
 @IsTest static void PR13(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle(null);n.setBody(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR13|'+JSON.serialize(o));}
 @IsTest static void PR14(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('bad-id');n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR14|'+JSON.serialize(o));}
 @IsTest static void PR15(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTitle(null);n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR15|'+JSON.serialize(o));}
 @IsTest static void PR16(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String u=String.valueOf(UserInfo.getUserId());String bad=u.substring(0,17)+(u.endsWith('A')?'B':'A');recipients=new Set<String>{bad};n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR16|'+JSON.serialize(o));}
 @IsTest static void PR17(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId('bad-id');n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR17|'+JSON.serialize(o));}
 @IsTest static void PR18(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String u=String.valueOf(UserInfo.getUserId());String bad=u.substring(0,17)+(u.endsWith('A')?'B':'A');recipients=new Set<String>{bad};n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR18|'+JSON.serialize(o));}
 @IsTest static void PR19(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String u=String.valueOf(UserInfo.getUserId());String bad=u.substring(0,17)+(u.endsWith('A')?'B':'A');recipients=new Set<String>{bad};n.setNotificationTypeId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR19|'+JSON.serialize(o));}
 @IsTest static void PR20(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String u=String.valueOf(UserInfo.getUserId());String bad=u.substring(0,17)+(u.endsWith('A')?'B':'A');recipients=new Set<String>{bad};n.setTargetId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR20|'+JSON.serialize(o));}
 @IsTest static void PR21(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetPageRef('{"type":"standard__namedPage","attributes":{"pageName":"home"}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR21|'+JSON.serialize(o));}
 @IsTest static void PR22(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('not-json');n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR22|'+JSON.serialize(o));}
 @IsTest static void PR23(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('not-json');recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR23|'+JSON.serialize(o));}
 @IsTest static void PR24(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('not-json');n.setNotificationTypeId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PR24|'+JSON.serialize(o));}
 @IsTest static void TP01(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('[]');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TP01|'+JSON.serialize(o));}
 @IsTest static void TP02(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('1');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TP02|'+JSON.serialize(o));}
 @IsTest static void TP03(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('x');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TP03|'+JSON.serialize(o));}
 @IsTest static void TP04(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('"x"');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TP04|'+JSON.serialize(o));}
 @IsTest static void TP05(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('null');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TP05|'+JSON.serialize(o));}
 @IsTest static void TP06(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TP06|'+JSON.serialize(o));}
 @IsTest static void TP07(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetPageRef('[]');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TP07|'+JSON.serialize(o));}
 @IsTest static void CS01(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String u=String.valueOf(UserInfo.getUserId());String bad=u.substring(0,17)+(u.endsWith('A')?'B':'A');n.setTargetId(bad);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|CS01|'+JSON.serialize(o));}
 @IsTest static void CS02(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String u=String.valueOf(UserInfo.getUserId());String bad=u.substring(0,17)+(u.endsWith('A')?'B':'A');n.setSenderId(bad);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|CS02|'+JSON.serialize(o));}
 @IsTest static void CS03(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(String.valueOf(UserInfo.getUserId()).substring(0,15));
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|CS03|'+JSON.serialize(o));}
 @IsTest static void OT01(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='OrgOnlyNotice' LIMIT 1].Id);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|OT01|'+JSON.serialize(o));}
 @IsTest static void OT02(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('0MLEc000001YkRFOA0');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|OT02|'+JSON.serialize(o));}
 @IsTest static void OT03(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId('0MLEc000001YkRFOA0');n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|OT03|'+JSON.serialize(o));}
 @IsTest static void PS01(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setNotificationTypeId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS01|'+JSON.serialize(o));}
 @IsTest static void PS02(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS02|'+JSON.serialize(o));}
 @IsTest static void PS03(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 String u=String.valueOf(UserInfo.getUserId());String bad=u.substring(0,17)+(u.endsWith('A')?'B':'A');n.setTargetId(null);recipients=new Set<String>{bad};
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS03|'+JSON.serialize(o));}
 @IsTest static void PS04(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);recipients.clear();for(Integer i=1;i<=501;i++)recipients.add('005'+String.valueOf(i).leftPad(12,'0'));
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS04|'+JSON.serialize(o));}
 @IsTest static void PS05(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients.clear();for(Integer i=1;i<=501;i++)recipients.add('005'+String.valueOf(i).leftPad(12,'0'));n.setNotificationTypeId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS05|'+JSON.serialize(o));}
 @IsTest static void PS06(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients.clear();for(Integer i=1;i<=501;i++)recipients.add('005'+String.valueOf(i).leftPad(12,'0'));n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS06|'+JSON.serialize(o));}
 @IsTest static void PS07(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients.clear();for(Integer i=1;i<=501;i++)recipients.add('005'+String.valueOf(i).leftPad(12,'0'));n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS07|'+JSON.serialize(o));}
 @IsTest static void PS08(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients.clear();for(Integer i=1;i<=501;i++)recipients.add('005'+String.valueOf(i).leftPad(12,'0'));n.setTargetId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS08|'+JSON.serialize(o));}
 @IsTest static void PS09(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setSenderId('bad-id');recipients.clear();
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS09|'+JSON.serialize(o));}
 @IsTest static void PS10(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setSenderId('bad-id');n.setBody(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS10|'+JSON.serialize(o));}
 @IsTest static void PS11(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId('bad-id');n.setTargetPageRef('not-json');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS11|'+JSON.serialize(o));}
 @IsTest static void PS12(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setNotificationTypeId(null);n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS12|'+JSON.serialize(o));}
 @IsTest static void PS13(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;n.setNotificationTypeId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS13|'+JSON.serialize(o));}
 @IsTest static void PS14(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;n.setTitle(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS14|'+JSON.serialize(o));}
 @IsTest static void PS15(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;n.setSenderId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS15|'+JSON.serialize(o));}
 @IsTest static void PS16(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;n.setTargetId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS16|'+JSON.serialize(o));}
 @IsTest static void PS17(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=new Set<String>{null};n.setNotificationTypeId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS17|'+JSON.serialize(o));}
 @IsTest static void PS18(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=new Set<String>{null};n.setTargetId('bad-id');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS18|'+JSON.serialize(o));}
 @IsTest static void PS19(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;n.setNotificationTypeId(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS19|'+JSON.serialize(o));}
 @IsTest static void PS20(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;n.setBody(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS20|'+JSON.serialize(o));}
 @IsTest static void PS21(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 recipients=null;n.setTargetId(null);
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|PS21|'+JSON.serialize(o));}
 @IsTest static void TQ01(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"a":1}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ01|'+JSON.serialize(o));}
 @IsTest static void TQ02(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"standard__namedPage"}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ02|'+JSON.serialize(o));}
 @IsTest static void TQ03(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"attributes":{"pageName":"home"}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ03|'+JSON.serialize(o));}
 @IsTest static void TQ04(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"standard__namedPage","attributes":{}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ04|'+JSON.serialize(o));}
 @IsTest static void TQ05(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":1}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ05|'+JSON.serialize(o));}
 @IsTest static void TQ06(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"x"}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ06|'+JSON.serialize(o));}
 @IsTest static void TQ07(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef(' {"type":"standard__namedPage","attributes":{"pageName":"home"}} ');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ07|'+JSON.serialize(o));}
 @IsTest static void TQ08(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ08|'+JSON.serialize(o));}
 @IsTest static void TQ09(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('true');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ09|'+JSON.serialize(o));}
 @IsTest static void TQ10(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"standard__namedPage","attributes":{"pageName":"home"},"state":{}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ10|'+JSON.serialize(o));}
 @IsTest static void TQ11(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":1,"attributes":{}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ11|'+JSON.serialize(o));}
 @IsTest static void TQ12(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"x","attributes":{}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ12|'+JSON.serialize(o));}
 @IsTest static void TQ13(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"standard__namedPage","attributes":[]}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ13|'+JSON.serialize(o));}
 @IsTest static void TQ14(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"standard__namedPage","attributes":null}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ14|'+JSON.serialize(o));}
 @IsTest static void TQ15(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"","attributes":{}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ15|'+JSON.serialize(o));}
 @IsTest static void TQ16(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":null,"attributes":{}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ16|'+JSON.serialize(o));}
 @IsTest static void TQ17(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"type":"standard__namedPage","attributes":"x"}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ17|'+JSON.serialize(o));}
 @IsTest static void TQ18(){Map<String,Object> o=new Map<String,Object>();
String stage='setup';
try {
 Messaging.CustomNotification n=new Messaging.CustomNotification();
 n.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
 n.setTitle('Probe title');n.setBody('Probe body');n.setTargetId(UserInfo.getUserId());
 Set<String> recipients=new Set<String>{UserInfo.getUserId()};
 stage='mutation';
 n.setTargetId(null);n.setTargetPageRef('{"TYPE":"standard__namedPage","ATTRIBUTES":{}}');
 stage='send';n.send(recipients);o.put('returned',true);
} catch(Exception e) {o.put('stage',stage);o.put('exceptionType',e.getTypeName());o.put('exceptionMessage',e.getMessage());}
System.assert(false,'OBS|TQ18|'+JSON.serialize(o));}
 @IsTest static void OPV1(){Object o=ViewAllFieldsObservation.observe('OPV1');
System.assert(false,'OBS|OPV1|'+JSON.serialize(o));}
 @IsTest static void OPV2(){Object o=ViewAllFieldsObservation.observe('OPV2');
System.assert(false,'OBS|OPV2|'+JSON.serialize(o));}
 @IsTest static void OPV3(){Object o=ViewAllFieldsObservation.observe('OPV3');
System.assert(false,'OBS|OPV3|'+JSON.serialize(o));}
 @IsTest static void OPV4(){Object o=ViewAllFieldsObservation.observe('OPV4');
System.assert(false,'OBS|OPV4|'+JSON.serialize(o));}
 @IsTest static void OPV5(){Object o=ViewAllFieldsObservation.observe('OPV5');
System.assert(false,'OBS|OPV5|'+JSON.serialize(o));}
 @IsTest static void OPV7(){Object o=ViewAllFieldsObservation.observe('OPV7');
System.assert(false,'OBS|OPV7|'+JSON.serialize(o));}
 @IsTest static void OPV8(){Object o=ViewAllFieldsObservation.observe('OPV8');
System.assert(false,'OBS|OPV8|'+JSON.serialize(o));}
 @IsTest static void OPV6(){Object o=ViewAllFieldsObservation.observe('OPV6');
System.assert(false,'OBS|OPV6|'+JSON.serialize(o));}
}
