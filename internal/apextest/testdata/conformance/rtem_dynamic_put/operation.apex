SObject e = new ReportEvent(); String m = 'accepted'; try {e.put('Operation', 'Export');} catch (SObjectException x) {m=x.getMessage();} System.debug('NATIVE|'+m);
