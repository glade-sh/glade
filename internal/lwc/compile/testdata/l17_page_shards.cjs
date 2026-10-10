// Export of the shared owned browser_capture.py NODE_PAGE_SHARDS transport.

async function familyprobePageShards(context, firstPage, rows, observe, serial=false, pageForRow=null){
 const count=Number(process.env.GLADE_BROWSER_PAGES||'3');
 if(![1,3].includes(count))throw new Error('BROWSER_PAGES_INVALID');
 const assignment=rows.map((row,index)=>count===1?0:pageForRow?pageForRow(row):index%count);
 if(assignment.some(index=>!Number.isInteger(index)||index<0||index>=count))throw new Error('BROWSER_PAGE_ASSIGNMENT_INVALID');
 const pages=[firstPage];
 for(let index=1;index<count;index++)pages.push(await context.newPage());
 const url=firstPage.url(); // Private authenticated location, never logged.
 await Promise.all(pages.slice(1).map(page=>page.goto(url,{waitUntil:'domcontentloaded',timeout:60000})));
 const fs=require('fs');let yielded=false;
 const pendingTrain=()=>{
  const e=process.env;
  if(!e.GLADE_QUEUE_DIR||!e.GLADE_QUEUE_HOST||!e.GLADE_QUEUE_JOB_ID||!e.GLADE_BROWSER_YIELD_FILE)return false;
  try{return fs.readdirSync(e.GLADE_QUEUE_DIR+'/pending').filter(name=>name.endsWith('.json')).some(name=>{
   try{const job=JSON.parse(fs.readFileSync(e.GLADE_QUEUE_DIR+'/pending/'+name,'utf8'));
    return job.host===e.GLADE_QUEUE_HOST&&/^TRAIN-(smoke|lane)-.+$/.test(job.family||'');
   }catch(_){return false;}
  });}catch(_){return false;}
 };
 const stop={_value:false,get value(){if(!this._value&&pendingTrain()){yielded=true;this._value=true;}return this._value;},set value(v){this._value=v;}};
 const work=async(page,index)=>{
  const scope={row:null};
  try{await observe(page,rows.filter((_,offset)=>assignment[offset]===index),index,stop,scope);}
  catch(_){
   stop.value=true;
   const row=/^[A-Za-z0-9_-]+$/.test(scope.row||'')?scope.row:'unavailable';
   process.stderr.write('browser-row page='+index+' row='+row+' reason=BROWSER_OPERATION_FAILED\n');
   throw new Error('BROWSER_OPERATION_FAILED');
  }
 };
 // Owned cookies are context-wide. V14 serializes its cookie-mutating shards.
 if(serial){for(const [index,page] of pages.entries()){if(stop.value)break;await work(page,index);}}
 else{
  const results=await Promise.allSettled(pages.map(work));
  if(results.some(result=>result.status==='rejected'))throw new Error('BROWSER_OPERATION_FAILED');
 }
 if(yielded){
  const e=process.env,path=e.GLADE_BROWSER_YIELD_FILE,temp=path+'.'+process.pid+'.tmp';
  fs.writeFileSync(temp,JSON.stringify({job_id:e.GLADE_QUEUE_JOB_ID,host:e.GLADE_QUEUE_HOST,reason:'pending_train'}));
  fs.renameSync(temp,path);process.stderr.write('[browser-yield]\n');
 }
}
// Draft for root-owned NODE_PAGE_SHARDS. No auth-bearing raw exception output.
async function familyprobeRowFailure(page,id,index,stage,error){
 const scrub=s=>String(s||'').replace(/https?:\/\/[^\s"'<>]+/gi,'<URL>')
  .replace(/\bBearer\s+\S+/gi,'Bearer <REDACTED>')
  .replace(/\b(?:access_?token|refresh_?token|client_?secret|sessionId|sfdxAuthUrl|password|authorization|sid|otp|frontdoor_uri)\s*["']?\s*[:=]\s*(?:"[^"]*"|'[^']*'|[^\s,;<>]+)/gi,'<REDACTED>')
  .replace(/\b00D[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?![A-Za-z0-9._-]+/g,'<REDACTED>');
 const allowed=['SCRATCH_LOGIN_UNAVAILABLE','RUNTIME_DEPLOYMENT_MISMATCH','HTTP_TRANSIENT','CAPTURE_PREREQUISITE_UNAVAILABLE'];
 let reason=allowed.includes(error?.message)?error.message:'BROWSER_OPERATION_FAILED';
 const diagnostics={};
 try{
  const dialog=page.locator('#auraErrorMask');
  if(await dialog.count()===1&&await dialog.isVisible()){
   reason='AURA_ERROR_DIALOG';diagnostics.aura= scrub(await dialog.innerText()).slice(0,4096);
  }
  diagnostics.nativeErrorText=scrub((await page.locator('#errorTitle,#errorBody,.errorMsg,.messageText,[role="alert"]').allTextContents()).join(' ')).slice(0,4096);
 }catch(_){}
 return {id,error:reason,page:index,stage,diagnostics};
}

module.exports = familyprobePageShards;
