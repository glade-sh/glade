@IsTest(SeeAllData=false) private class DootAY5A1031Test {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}
 @IsTest static void finishSchedules(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y5.finishSchedules','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY5A1031Wire.fixture();  d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id job=Database.executeBatch(new DootAY5A1031Parent(owned),1);d.put('job',job);d.put('submitted',job!=null);}catch(Exception e){DootAY5A1031Wire.error(d,e,'submit');}
  d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');
 }catch(Exception e){DootAY5A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY5A1031Wire.read(owned));}catch(Exception e){DootAY5A1031Wire.error(d,e,'owned-read');}}
 DootAY5A1031Wire.emit('Y5.finishSchedules',d);
 }
 @IsTest static void directScheduleControl(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y5.control','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY5A1031Wire.fixture();  d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id cron=System.scheduleBatch(new DootAY5A1031Child(),'DootY5Ctrl-'+String.valueOf(owned),1);d.put('cron',cron);d.put('submitted',cron!=null);}catch(Exception e){DootAY5A1031Wire.error(d,e,'submit');}
 DootAY5A1031Wire.put(owned,new Map<String,Object>{'cb'=>'Y5.directSchedule','stage'=>'complete'}); d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');
 }catch(Exception e){DootAY5A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY5A1031Wire.read(owned));}catch(Exception e){DootAY5A1031Wire.error(d,e,'owned-read');}}
 DootAY5A1031Wire.emit('Y5.control',d);
 }

}
