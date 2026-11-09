for (Id key : new List<Id>{null,Id.valueOf('00O000000000001'),Id.valueOf('001000000000001')}) {
 String token=key==null?'null':String.valueOf(key).substring(0,3);
 try { Reports.ReportManager.runReport(key,(Reports.ReportMetadata)null); System.debug('B6REJECT2|metadata|'+token+'|OK'); } catch(Exception e){System.debug('B6REJECT2|metadata|'+token+'|'+e.getTypeName()+'|'+e.getMessage());}
 try { Reports.ReportManager.runReport(key,(Reports.ReportMetadata)null,false); System.debug('B6REJECT2|metadata-false|'+token+'|OK'); } catch(Exception e){System.debug('B6REJECT2|metadata-false|'+token+'|'+e.getTypeName()+'|'+e.getMessage());}
}
