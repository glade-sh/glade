public class TaskFlagCapture {
 public static List<Object> events=new List<Object>();
 public static Object readFlag(Task row,String mode){
  try {
   Object value;
   if(mode=='direct')value=row.IsHighPriority;
   else if(mode=='string')value=row.get('IsHighPriority');
   else value=row.get(Task.IsHighPriority);
   return new Map<String,Object>{'ok'=>true,'value'=>value};
  } catch(Exception e){return new Map<String,Object>{'ok'=>false,'type'=>e.getTypeName(),'message'=>e.getMessage()};}
 }
 public static Map<String,Object> snapshot(Task row){
  Object priority;
  try {priority=row.Priority;}catch(Exception e){priority=new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage()};}
  return new Map<String,Object>{'priority'=>priority,'direct'=>readFlag(row,'direct'),'string'=>readFlag(row,'string'),'token'=>readFlag(row,'token')};
 }
 public static void recordPhase(String phase,List<Task> rows,List<Task> oldRows){
  for(Integer i=0;i<rows.size();i++){
   Map<String,Object> event=new Map<String,Object>{'phase'=>phase,'new'=>snapshot(rows[i])};
   if(oldRows!=null)event.put('old',snapshot(oldRows[i]));
   events.add(event);
  }
 }
 public static Map<String,Object> observe(String rowId){
  events=new List<Object>();
  List<TaskPriority> setup=[SELECT ApiName,MasterLabel,IsDefault,IsHighPriority FROM TaskPriority ORDER BY SortOrder];
  List<Object> priorities=new List<Object>();
  Boolean customReady=false;
  for(TaskPriority p:setup){
   priorities.add(new Map<String,Object>{'apiName'=>p.ApiName,'label'=>p.MasterLabel,'default'=>p.IsDefault,'high'=>p.IsHighPriority});
   if(p.ApiName=='Pulse'&&p.IsHighPriority)customReady=true;
  }
  Map<String,Object> result=new Map<String,Object>{'row'=>rowId,'priorities'=>priorities};
  if(rowId=='T06'&&!customReady){result.put('outcome','setupUnavailable');return result;}
  Task item=new Task(Subject='Flag observation');
  if(rowId=='T01'||rowId=='T04'||rowId=='T08')item.Priority='High';
  else if(rowId=='T02')item.Priority='Normal';
  else if(rowId=='T03'||rowId=='T07')item.Priority='Low';
  else if(rowId=='T06')item.Priority='Pulse';
  result.put('beforeInsertCaller',snapshot(item));
  try {
   insert item;
   result.put('afterInsertCaller',snapshot(item));
   Task selected=[SELECT Id,Priority,IsHighPriority FROM Task WHERE Id=:item.Id];
   result.put('afterInsertQuery',snapshot(selected));
   Task omitted=[SELECT Id,Priority FROM Task WHERE Id=:item.Id];
   result.put('omission',snapshot(omitted));
   if(rowId=='T04'||rowId=='T07'||rowId=='T08'){
    if(rowId=='T08')item=new Task(Id=item.Id,Subject='Sparse flag observation');
    else item.Priority=rowId=='T04'?'Low':'High';
    result.put('beforeUpdateCaller',snapshot(item));
    update item;
    result.put('afterUpdateCaller',snapshot(item));
    selected=[SELECT Id,Priority,IsHighPriority FROM Task WHERE Id=:item.Id];
    result.put('afterUpdateQuery',snapshot(selected));
    omitted=[SELECT Id,Priority FROM Task WHERE Id=:item.Id];
    result.put('afterUpdateOmission',snapshot(omitted));
   }
   result.put('outcome','complete');
  }catch(Exception e){result.put('outcome','exception');result.put('exception',new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage()});}
  result.put('triggers',events);
  return result;
 }
 public static void emit(String rowId,Integer slot){
  String payload=EncodingUtil.base64Encode(Blob.valueOf(JSON.serialize(observe(rowId))));
  String digest=EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256',Blob.valueOf(payload)));
  Integer parts=(payload.length()+299)/300;
  Integer start=slot*300;
  String part=start<payload.length()?payload.substring(start,Math.min(start+300,payload.length())):'';
  System.assert(false,'TASK_FLAG|'+rowId+'|'+slot+'|'+parts+'|'+digest+'|'+part);
 }
}
