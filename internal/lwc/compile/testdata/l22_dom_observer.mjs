export default node=>{
 const seen=new Set(),overlays=[];
 function walk(root){
  if(!root || seen.has(root))return;seen.add(root);
  for(const e of root.querySelectorAll('*')){
   const visible=e.getClientRects().length>0 && getComputedStyle(e).visibility!=='hidden';
   if(visible && e.matches('.slds-modal,[role="dialog"],[role="alertdialog"],.slds-notify_toast')){
    overlays.push({tag:e.tagName,role:e.getAttribute('role'),label:e.getAttribute('aria-label'),labelledByPresent:!!e.getAttribute('aria-labelledby'),descriptionPresent:!!e.getAttribute('aria-describedby'),text:e.textContent,
     buttons:Array.from(e.querySelectorAll('button')).map(b=>({text:b.textContent,label:b.getAttribute('aria-label'),title:b.title,disabled:b.disabled})),
     fields:Array.from(e.querySelectorAll('input,textarea')).map(f=>({type:f.type,value:f.value,disabled:f.disabled})),shadowAccessible:!!e.shadowRoot});
   }
   if(e.shadowRoot)walk(e.shadowRoot);
  }
 }
 walk(document);
 const active=document.activeElement;
 return {overlays,activeElement:active?{tag:active.tagName,role:active.getAttribute('role'),label:active.getAttribute('aria-label')}:null,accessibleDOMOnly:true,observation:overlays.length?'visible overlay DOM observed':'no accessible overlay DOM at this checkpoint'};
};
