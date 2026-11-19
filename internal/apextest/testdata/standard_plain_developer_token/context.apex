System.debug('CONTEXT=' + UserInfo.getLocale() + ':' + [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name);
