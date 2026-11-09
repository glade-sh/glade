for (Id key : new List<Id>{null,Id.valueOf('00O000000000001'),Id.valueOf('001000000000001')}) {
 String token=key==null?'null':String.valueOf(key).substring(0,3);
 try { Reports.ReportManager.describeReport(key); System.debug('B6REJECT|describe|'+token+'|OK'); } catch(Exception e){System.debug('B6REJECT|describe|'+token+'|'+e.getTypeName()+'|'+e.getMessage());}
 try { Reports.ReportManager.runReport(key,true); System.debug('B6REJECT|run|'+token+'|OK'); } catch(Exception e){System.debug('B6REJECT|run|'+token+'|'+e.getTypeName()+'|'+e.getMessage());}
}
