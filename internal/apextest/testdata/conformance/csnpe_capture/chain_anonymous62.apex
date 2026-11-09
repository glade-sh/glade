ProbeHolder h = new ProbeHolder();
try { String x=h.setting.Text__c; System.debug('CHAIN_VALUE='+x); } catch (Exception e) { System.debug('CHAIN_EXCEPTION='+e.getTypeName()+'|'+e.getMessage()+'|'+e.getLineNumber()); }
Account a=new Account();
try { String x=a.Parent.Name; System.debug('REL_VALUE='+x); } catch (Exception e) { System.debug('REL_EXCEPTION='+e.getTypeName()+'|'+e.getMessage()+'|'+e.getLineNumber()); }
System.debug('NAMED_START');
ChainProbe62.run();
