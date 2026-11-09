User u=[SELECT Id,TimeZoneSidKey FROM User WHERE Id=:UserInfo.getUserId()];u.TimeZoneSidKey='Pacific/Kiritimati';update u;
