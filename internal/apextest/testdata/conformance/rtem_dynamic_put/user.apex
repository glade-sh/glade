SObject e = new ReportEvent(); String m = 'accepted'; try {e.put(ReportEvent.UserId, UserInfo.getUserId());} catch (SObjectException x) {m=x.getMessage();} System.debug('NATIVE|'+m);
