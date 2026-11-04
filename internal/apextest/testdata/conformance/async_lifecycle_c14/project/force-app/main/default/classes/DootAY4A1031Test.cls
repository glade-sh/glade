@IsTest(SeeAllData=false) private class DootAY4A1031Test {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}
 @IsTest static void twoChunks(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y4.twoChunks','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY4A1031Wire.fixture();  d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id job=Database.executeBatch(new DootAY4A1031Batch(owned,true),1);d.put('job',job);d.put('submitted',job!=null);}catch(Exception e){DootAY4A1031Wire.error(d,e,'submit');}
  d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');
 }catch(Exception e){DootAY4A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY4A1031Wire.read(owned));}catch(Exception e){DootAY4A1031Wire.error(d,e,'owned-read');}}
 DootAY4A1031Wire.emit('Y4.twoChunks',d);
 }
 @IsTest static void oneChunkControl(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y4.control','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY4A1031Wire.fixture();  d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id job=Database.executeBatch(new DootAY4A1031Batch(owned,false),1);d.put('job',job);d.put('submitted',job!=null);}catch(Exception e){DootAY4A1031Wire.error(d,e,'submit');}
  d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');
 }catch(Exception e){DootAY4A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY4A1031Wire.read(owned));}catch(Exception e){DootAY4A1031Wire.error(d,e,'owned-read');}}
 DootAY4A1031Wire.emit('Y4.control',d);
 }

}
