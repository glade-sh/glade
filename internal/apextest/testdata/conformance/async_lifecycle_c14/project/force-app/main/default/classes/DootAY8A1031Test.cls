@IsTest(SeeAllData=false) private class DootAY8A1031Test {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}
 @IsTest static void oversizedIterable(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y8.oversized','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY8A1031Wire.fixture();  d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id cron=System.scheduleBatch(new DootAY8A1031Batch(owned),'DootY8-'+String.valueOf(owned),1,2001);d.put('cron',cron);d.put('submitted',cron!=null);}catch(Exception e){DootAY8A1031Wire.error(d,e,'submit');}
 if(d.containsKey('cron')){Id cron=(Id)d.get('cron');d.put('cronRows',[SELECT Id,State FROM CronTrigger WHERE Id=:cron WITH SYSTEM_MODE]);} d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');
 }catch(Exception e){DootAY8A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY8A1031Wire.read(owned));}catch(Exception e){DootAY8A1031Wire.error(d,e,'owned-read');}}
 DootAY8A1031Wire.emit('Y8.oversized',d);
 }
 @IsTest static void scheduledControl(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y8.control','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY8A1031Wire.fixture();  d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id cron=System.scheduleBatch(new DootAY8A1031Batch(owned),'DootY8Ctrl-'+String.valueOf(owned),1,1);d.put('cron',cron);d.put('submitted',cron!=null);}catch(Exception e){DootAY8A1031Wire.error(d,e,'submit');}
 if(d.containsKey('cron')){Id cron=(Id)d.get('cron');d.put('cronRows',[SELECT Id,State FROM CronTrigger WHERE Id=:cron WITH SYSTEM_MODE]);} d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');
 }catch(Exception e){DootAY8A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY8A1031Wire.read(owned));}catch(Exception e){DootAY8A1031Wire.error(d,e,'owned-read');}}
 DootAY8A1031Wire.emit('Y8.control',d);
 }

}
