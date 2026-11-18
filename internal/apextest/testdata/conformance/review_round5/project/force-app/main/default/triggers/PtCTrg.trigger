trigger PtCTrg on PtC__c (after insert) { String v;
 try { PtC__c c = Trigger.new[0]; v = c.P__r == null ? 'null' : String.valueOf(c.P__r.Cnt__c); } catch (Exception e) { v = e.getTypeName(); }
 PtLog.seen += v + ';'; }
