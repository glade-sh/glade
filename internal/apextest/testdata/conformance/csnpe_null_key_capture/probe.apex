delete [SELECT Id FROM CsnpeProbe__c]; String key; insert new CsnpeProbe__c(Name='Present',Source__c='value');
try { CsnpeProbe__c r=CsnpeProbe__c.getValues(key); System.debug('CSNPE null_row='+(r==null)); System.debug('CSNPE null_id_is_null='+(r.Id==null)); } catch(Exception e) {System.debug('CSNPE null_id_exception='+e.getTypeName());}
try { CsnpeProbe__c r=CsnpeProbe__c.getValues(key); System.debug('CSNPE null_field='+r.Source__c); } catch(Exception e) {System.debug('CSNPE null_field_exception='+e.getTypeName());}
try { CsnpeProbe__c r=CsnpeProbe__c.getValues('absent'); System.debug('CSNPE absent_row='+(r==null)); System.debug('CSNPE absent_id_is_null='+(r.Id==null)); } catch(Exception e) {System.debug('CSNPE absent_id_exception='+e.getTypeName());}
try { CsnpeProbe__c r=CsnpeProbe__c.getValues('absent'); System.debug('CSNPE absent_field='+r.Source__c); } catch(Exception e) {System.debug('CSNPE absent_field_exception='+e.getTypeName());}
try { CsnpeProbe__c r=CsnpeProbe__c.getValues('Present'); System.debug('CSNPE present_field='+r.Source__c); } catch(Exception e) {System.debug('CSNPE present_exception='+e.getTypeName());}
