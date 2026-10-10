// Owned native action sequence, replayed locally without expected answers.
export default async function overlayAction(page, spec) {
 const req={op:"overlay-action",frame:0,service:spec.service,action:spec.action,typed:spec.typed,observeRefusedDismiss:["r_modal_disable_true_dismiss","r_modal_disable_string_false_dismiss","r_modal_disable_number_dismiss"].includes(spec.id)};
 const safe=s=>String(s||"").replace(/https?:\/\/[^\s"'<>]+/gi,"<URL>");

 if(req.op==='overlay-action' && req.observeRefusedDismiss && req.service==='modal' && req.action==='dismiss'){
  const frame=page.frames()[req.frame];
  const roots=()=>frame.locator('.slds-modal:visible, [role="dialog"]:visible, [role="alertdialog"]:visible');
  const started=Date.now();let root;
  while(Date.now()-started<4000){
   const nodes=roots();if(await nodes.count()){root=nodes.last();break;}
   await new Promise(r=>setTimeout(r,100));
  }
  if(!root)return {observation:{kind:'bounded-no-overlay',appearanceWindowMs:4000,action:req.action,performed:false}};
  const observation={kind:'overlay-observed',appearanceWindowMs:4000,action:req.action,attempted:false,performed:false,before:await root.innerText({timeout:2000})};
  // Preserve the predecessor's native close selector and never force a click.
  const control=frame.locator('button[title="Close"], button[aria-label="Close"], .slds-modal__close, .slds-notify__close button').last();
  const controlStarted=Date.now();
  while(Date.now()-controlStarted<2000 && (!await control.count() || !await control.isVisible()))await new Promise(r=>setTimeout(r,100));
  if(!await control.count() || !await control.isVisible()){
   observation.controlObservation='bounded-no-action-control';observation.controlWindowMs=2000;
  }else{
   observation.attempted=true;
   observation.controlDisabled=!await control.isEnabled();
   if(observation.controlDisabled)observation.controlObservation='disabled';
   else{
    try{await control.click({timeout:3000});observation.performed=true;}
    catch(error){
     // Only native actionability refusal is an answer. Detached contexts,
     // closed pages, network errors and unrelated bridge errors still fail.
     const text=String(error.message||'');
     if(error.name!=='TimeoutError' || !/not enabled|disabled|intercepts pointer events|does not receive pointer events|not receiving pointer/i.test(text))throw error;
     observation.actionError={name:error.name,message:safe(text)};
     observation.controlObservation='actionability-refused-within-3000ms';
    }
   }
  }
  const settled=Date.now();let stillOpen=await root.isVisible();
  while(stillOpen && Date.now()-settled<1000){
   await new Promise(r=>setTimeout(r,100));stillOpen=await root.isVisible();
  }
  observation.observationWindowMs=1000;observation.stillOpen=stillOpen;
  observation.kind=stillOpen?(observation.controlObservation==='bounded-no-action-control'?'bounded-no-action-control':'bounded-dismiss-refused'):'dismissed-within-window';
  observation.errorText=stillOpen?(await root.locator('[role="alert"]:visible, .slds-form-element__help:visible, .errorMsg:visible').allTextContents()).map(safe):[];
  if(stillOpen)observation.after=await root.innerText({timeout:2000});
  return {observation};
 }

 if(req.op==='overlay-action'){
  const frame=page.frames()[req.frame];
  const roots=()=>req.service==='toast'?frame.locator('.slds-notify_toast:visible, [role="alert"]:visible').filter({hasText:'L22 Owned'}):frame.locator('.slds-modal:visible, [role="dialog"]:visible, [role="alertdialog"]:visible');
  const started=Date.now();let root;
  while(Date.now()-started<4000){
   const nodes=roots();if(await nodes.count()){root=nodes.last();break;}
   await new Promise(r=>setTimeout(r,100));
  }
  if(!root)return {observation:{kind:'bounded-no-overlay',appearanceWindowMs:4000,action:req.action,performed:false}};
  const before=await root.innerText({timeout:2000});
  const observation={kind:'overlay-observed',appearanceWindowMs:4000,action:req.action,performed:false,before};
  if(req.action==='observe'){
   await new Promise(r=>setTimeout(r,req.service==='toast'?6000:800));
   observation.observationWindowMs=req.service==='toast'?6000:800;
   observation.visibleAfter=await root.isVisible().catch(()=>false);return {observation};
  }
  if(req.action==='escape'){
   await page.keyboard.press('Escape');
   observation.performed=true;return {observation};
  }
  if(req.service==='prompt' && req.typed!==null && req.typed!==undefined){
   const field=root.locator('input:not([type="hidden"]), textarea').first();
   if(!await field.count())return {observation:{...observation,kind:'bounded-no-input',controlWindowMs:0}};
   await field.fill(req.typed,{timeout:2000});observation.typed=req.typed;
  }
  let control;
  if(req.service==='modal' && req.action.startsWith('close_'))control=frame.locator('[data-l22-choice]:visible').last();
  else if(req.action==='dismiss')control=(req.service==='modal'?frame:root).locator('button[title="Close"], button[aria-label="Close"], .slds-modal__close, .slds-notify__close button').last();
  else if(req.action==='choice')control=root.getByRole('link',{name:'Owned choice',exact:true});
  else if(req.action==='cancel')control=root.getByRole('button',{name:/^Cancel$/i});
  else control=root.getByRole('button',{name:/^(OK|Okay|Confirm)$/i});
  const controlStarted=Date.now();
  while(Date.now()-controlStarted<2000 && (!await control.count() || !await control.last().isVisible()))await new Promise(r=>setTimeout(r,100));
  if(!await control.count() || !await control.last().isVisible())return {observation:{...observation,kind:'bounded-no-action-control',controlWindowMs:2000}};
  if(!await control.last().isEnabled())return {observation:{...observation,kind:'action-control-disabled'}};
  await control.last().click({timeout:3000});observation.performed=true;
  if(req.action==='choice')observation.choiceHash=await frame.evaluate(()=>location.hash);
  return {observation};
 }
}
