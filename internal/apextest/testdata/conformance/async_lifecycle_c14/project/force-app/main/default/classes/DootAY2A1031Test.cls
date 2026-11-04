@IsTest(SeeAllData=false) private class DootAY2A1031Test {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}
 @IsTest static void fromStart(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y2.fromStart','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY2A1031Wire.fixture(); Id child=DootAY2A1031Wire.fixture(); d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id job=Database.executeBatch(new DootAY2A1031Parent(owned,child,true),1);d.put('job',job);d.put('submitted',job!=null);}catch(Exception e){DootAY2A1031Wire.error(d,e,'submit');}
  d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');d.put('child',DootAY2A1031Wire.read(child));
 }catch(Exception e){DootAY2A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY2A1031Wire.read(owned));}catch(Exception e){DootAY2A1031Wire.error(d,e,'owned-read');}}
 DootAY2A1031Wire.emit('Y2.fromStart',d);
 }
 @IsTest static void fromExecute(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y2.fromExecute','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY2A1031Wire.fixture(); Id child=DootAY2A1031Wire.fixture(); d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id job=Database.executeBatch(new DootAY2A1031Parent(owned,child,false),1);d.put('job',job);d.put('submitted',job!=null);}catch(Exception e){DootAY2A1031Wire.error(d,e,'submit');}
  d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');d.put('child',DootAY2A1031Wire.read(child));
 }catch(Exception e){DootAY2A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY2A1031Wire.read(owned));}catch(Exception e){DootAY2A1031Wire.error(d,e,'owned-read');}}
 DootAY2A1031Wire.emit('Y2.fromExecute',d);
 }
 @IsTest static void childControl(){
 Map<String,Object> d=new Map<String,Object>{'id'=>'Y2.childControl','stage'=>'fixture','submitted'=>false,'stop'=>false};Id owned;
 try{owned=DootAY2A1031Wire.fixture();  d.put('stage','start');Test.startTest();
 try{d.put('stage','submit');Id job=Database.executeBatch(new DootAY2A1031Child(owned),1);d.put('job',job);d.put('submitted',job!=null);}catch(Exception e){DootAY2A1031Wire.error(d,e,'submit');}
  d.put('stage','stop');Test.stopTest();d.put('stop',true);d.put('stage','stopReturned');
 }catch(Exception e){DootAY2A1031Wire.error(d,e,String.valueOf(d.get('stage')));}
 if(owned!=null){try{d.put('cb',DootAY2A1031Wire.read(owned));}catch(Exception e){DootAY2A1031Wire.error(d,e,'owned-read');}}
 DootAY2A1031Wire.emit('Y2.childControl',d);
 }

}
