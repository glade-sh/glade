String typeId=[SELECT Id FROM CustomNotificationType WHERE DeveloperName='SyntheticNotice' LIMIT 1].Id;
for(String row:new List<String>{'analytics_enum','analytics_id','analytics_uuid','analytics_null','analytics_string','analytics_boolean','analytics_object','analytics_warn','analytics_warn_id','analytics_info_id','analytics_number_id','notice_missing_type','notice_missing_target','notice_missing_title','notice_empty_title','notice_bad_type','notice_configured','notice_page','notice_constructor'}) {
 String observed;
 try {
 if(row=='analytics_enum') { IsvPartners.AppAnalytics.logCustomInteraction(HostedBoundaryProbe.Gesture.Opened); }
 else if(row=='analytics_id') { IsvPartners.AppAnalytics.logCustomInteraction(HostedBoundaryProbe.Gesture.Opened, UserInfo.getUserId()); }
 else if(row=='analytics_uuid') { IsvPartners.AppAnalytics.logCustomInteraction(HostedBoundaryProbe.Gesture.Opened, UUID.randomUUID()); }
 else if(row=='analytics_null') { IsvPartners.AppAnalytics.logCustomInteraction(null); }
 else if(row=='analytics_string') { IsvPartners.AppAnalytics.logCustomInteraction('Opened'); }
 else if(row=='analytics_boolean') { IsvPartners.AppAnalytics.logCustomInteraction(true); }
 else if(row=='analytics_object') { IsvPartners.AppAnalytics.logCustomInteraction(new Account()); }
 else if(row=='analytics_warn') { IsvPartners.AppAnalytics.logCustomInteraction(System.LoggingLevel.WARN); }
 else if(row=='analytics_warn_id') { IsvPartners.AppAnalytics.logCustomInteraction(System.LoggingLevel.WARN, (Id)null); }
 else if(row=='analytics_info_id') { IsvPartners.AppAnalytics.logCustomInteraction(System.LoggingLevel.INFO, (Id)null); }
 else if(row=='analytics_number_id') { IsvPartners.AppAnalytics.logCustomInteraction(1, (Id)null); }
 else if(row=='notice_missing_type') { Messaging.CustomNotification n=new Messaging.CustomNotification();  n.send(new Set<String>{UserInfo.getUserId()}); }
 else if(row=='notice_missing_target') { Messaging.CustomNotification n=new Messaging.CustomNotification(); n.setNotificationTypeId(typeId); n.setTitle('Synthetic title'); n.setBody('Synthetic body'); n.send(new Set<String>{UserInfo.getUserId()}); }
 else if(row=='notice_missing_title') { Messaging.CustomNotification n=new Messaging.CustomNotification(); n.setNotificationTypeId(typeId); n.setBody('Synthetic body'); n.setTargetId(UserInfo.getUserId()); n.send(new Set<String>{UserInfo.getUserId()}); }
 else if(row=='notice_empty_title') { Messaging.CustomNotification n=new Messaging.CustomNotification(); n.setNotificationTypeId(typeId); n.setTitle(''); n.setBody('Synthetic body'); n.setTargetId(UserInfo.getUserId()); n.send(new Set<String>{UserInfo.getUserId()}); }
 else if(row=='notice_bad_type') { Messaging.CustomNotification n=new Messaging.CustomNotification(); n.setNotificationTypeId('invalid-type'); n.setTitle('Synthetic title'); n.setBody('Synthetic body'); n.setTargetId(UserInfo.getUserId()); n.send(new Set<String>{UserInfo.getUserId()}); }
 else if(row=='notice_configured') { Messaging.CustomNotification n=new Messaging.CustomNotification(); n.setNotificationTypeId(typeId); n.setSenderId(UserInfo.getUserId()); n.setTitle('Synthetic title'); n.setBody('Synthetic body'); n.setTargetId(UserInfo.getUserId()); n.send(new Set<String>{UserInfo.getUserId()}); }
 else if(row=='notice_page') { Messaging.CustomNotification n=new Messaging.CustomNotification(); n.setNotificationTypeId(typeId); n.setTitle('Synthetic title'); n.setBody('Synthetic body'); n.setTargetPageRef('{"type":"standard__namedPage","attributes":{"pageName":"home"}}'); n.send(new Set<String>{UserInfo.getUserId()}); }
 else if(row=='notice_constructor') { Messaging.CustomNotification n=new Messaging.CustomNotification(); n=new Messaging.CustomNotification(typeId, UserInfo.getUserId(), 'Synthetic title', 'Synthetic body', UserInfo.getUserId(), null); n.send(new Set<String>{UserInfo.getUserId()}); }
 observed='returned'; } catch(Exception e) { observed=e.getTypeName()+'|'+e.getMessage(); }
 System.debug('OBSERVED|'+row+'|'+JSON.serialize(new Map<String,Object>{'result'=>observed}));
}
