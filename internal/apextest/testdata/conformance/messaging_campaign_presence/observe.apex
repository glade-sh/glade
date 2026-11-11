Schema.SObjectType t=Schema.getGlobalDescribe().get('MessagingSession'); System.debug('CAP|'+String.valueOf(t!=null && t.getDescribe().fields.getMap().containsKey('CampaignId')));
