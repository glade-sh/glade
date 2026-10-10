// Native control projections/actions from owned capture exports; no oracle answers.
export const observeInteractionDOM = root=>{
 const nodes=[];
 function visit(el){
  for(const child of el.children || []){nodes.push(child);visit(child);if(child.shadowRoot)visit(child.shadowRoot);}
 }
 visit(root);
 const seen=new Set();
 const fieldName=n=>typeof n.fieldName==='string'?n.fieldName:n.fieldName&&n.fieldName.fieldApiName;
 const fields=nodes.filter(n=>['LIGHTNING-INPUT-FIELD','LIGHTNING-OUTPUT-FIELD'].includes(n.tagName)).map(n=>({component:n.tagName.toLowerCase(),field:fieldName(n),text:n.innerText||n.textContent||''}));
 const supported=new Set(['Name','LastName','Phone','Website','AnnualRevenue','NumberOfEmployees','Description','Industry','Email','Birthdate','AccountId','IsDeleted']);
 const ownedFields=nodes.filter(n=>['LIGHTNING-INPUT-FIELD','LIGHTNING-OUTPUT-FIELD'].includes(n.tagName)&&supported.has(fieldName(n)));
 const projected=new Set();for(const field of ownedFields){projected.add(field);const local=[];function walk(n){for(const c of n.children||[]){projected.add(c);walk(c);if(c.shadowRoot)walk(c.shadowRoot);}}walk(field);if(field.shadowRoot)walk(field.shadowRoot);}
 const controls=nodes.filter(n=>['INPUT','TEXTAREA','SELECT'].includes(n.tagName)).filter(n=>{if(root.dataset.component!=='picker'&&!projected.has(n))return false;if(seen.has(n))return false;seen.add(n);return true;}).map(n=>({tag:n.tagName.toLowerCase(),type:n.type,value:n.value,checked:n.checked,disabled:n.disabled,required:n.required,readOnly:n.readOnly,placeholder:n.placeholder,invalid:n.getAttribute('aria-invalid')}));
 const buttons=nodes.filter(n=>n.tagName==='BUTTON').map(n=>({text:n.innerText||n.textContent||'',title:n.title,disabled:n.disabled}));
 const errors=nodes.filter(n=>n.getAttribute('role')==='alert'||(n.classList && n.classList.contains('slds-form-element__help'))).map(n=>n.innerText||n.textContent||'');
 return {recordFormPresent:nodes.some(n=>['LIGHTNING-RECORD-FORM','LIGHTNING-RECORD-EDIT-FORM','LIGHTNING-RECORD-VIEW-FORM'].includes(n.tagName)),fields:fields.filter(f=>supported.has(f.field)),controls,buttons,errors};
};
export const selectInteractionPicklist = async(root,arg)=>{
 const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
 function all(parent){
 const result=[],seen=new Set();
 function visit(el){for(const child of el.children||[]){
  if(seen.has(child))continue;
  seen.add(child);result.push(child);visit(child);
  if(child.shadowRoot)visit(child.shadowRoot);
 }}
 visit(parent);
 // The owned lightning-input-field is itself a shadow host.
 if(parent.shadowRoot)visit(parent.shadowRoot);
 return result;
}
 const fields=[...new Set(all(root).filter(node=>node.tagName==='LIGHTNING-INPUT-FIELD'))];
 if(fields.length!==1)throw new Error('L19_CONTROL_PICKLIST_FIELD_NOT_UNIQUE');
 const field=fields[0];
 const trigger=all(field).find(node=>node.tagName==='BUTTON'&&!node.disabled);
 if(!trigger)throw new Error('L19_CONTROL_PICKLIST_TRIGGER_UNOBSERVED');
 const text=node=>node.innerText||node.textContent||'';
 const beforeValue=field.value,triggerText=text(trigger);
 trigger.click();
 let options=[];
 for(let attempt=0;attempt<30;attempt++){
  options=[...new Set(all(field).filter(node=>node.getAttribute('role')==='option'))];
  if(options.length)break;
  await sleep(100);
 }
 if(!options.length)throw new Error('L19_CONTROL_PICKLIST_OPTIONS_UNOBSERVED');
 const shape=node=>({tag:node.tagName.toLowerCase(),role:node.getAttribute('role'),text:text(node),
  selected:node.getAttribute('aria-selected'),disabled:node.getAttribute('aria-disabled'),value:node.getAttribute('data-value')});
 const opened=await (root=>{
 const nodes=[];
 function visit(el){
  for(const child of el.children || []){nodes.push(child);visit(child);if(child.shadowRoot)visit(child.shadowRoot);}
 }
 visit(root);
 const seen=new Set();
 const fieldName=n=>typeof n.fieldName==='string'?n.fieldName:n.fieldName&&n.fieldName.fieldApiName;
 const fields=nodes.filter(n=>['LIGHTNING-INPUT-FIELD','LIGHTNING-OUTPUT-FIELD'].includes(n.tagName)).map(n=>({component:n.tagName.toLowerCase(),field:fieldName(n),text:n.innerText||n.textContent||''}));
 const supported=new Set(['Name','LastName','Phone','Website','AnnualRevenue','NumberOfEmployees','Description','Industry','Email','Birthdate','AccountId','IsDeleted']);
 const ownedFields=nodes.filter(n=>['LIGHTNING-INPUT-FIELD','LIGHTNING-OUTPUT-FIELD'].includes(n.tagName)&&supported.has(fieldName(n)));
 const projected=new Set();for(const field of ownedFields){projected.add(field);const local=[];function walk(n){for(const c of n.children||[]){projected.add(c);walk(c);if(c.shadowRoot)walk(c.shadowRoot);}}walk(field);if(field.shadowRoot)walk(field.shadowRoot);}
 const controls=nodes.filter(n=>['INPUT','TEXTAREA','SELECT'].includes(n.tagName)).filter(n=>{if(root.dataset.component!=='picker'&&!projected.has(n))return false;if(seen.has(n))return false;seen.add(n);return true;}).map(n=>({tag:n.tagName.toLowerCase(),type:n.type,value:n.value,checked:n.checked,disabled:n.disabled,required:n.required,readOnly:n.readOnly,placeholder:n.placeholder,invalid:n.getAttribute('aria-invalid')}));
 const buttons=nodes.filter(n=>n.tagName==='BUTTON').map(n=>({text:n.innerText||n.textContent||'',title:n.title,disabled:n.disabled}));
 const errors=nodes.filter(n=>n.getAttribute('role')==='alert'||(n.classList && n.classList.contains('slds-form-element__help'))).map(n=>n.innerText||n.textContent||'');
 return {recordFormPresent:nodes.some(n=>['LIGHTNING-RECORD-FORM','LIGHTNING-RECORD-EDIT-FORM','LIGHTNING-RECORD-VIEW-FORM'].includes(n.tagName)),fields:fields.filter(f=>supported.has(f.field)),controls,buttons,errors};
})(root);
 const choice=options.find(node=>node.getAttribute('aria-disabled')!=='true'&&
  node.getAttribute('aria-selected')!=='true'&&text(node)!==text(trigger));
 if(!choice)throw new Error('L19_CONTROL_PICKLIST_ALTERNATIVE_UNOBSERVED');
 const clickedOption=shape(choice), observedOptions=options.map(shape);
 choice.click();await sleep(300);
 return {operation:arg.operation,beforeValue,triggerText,opened,options:observedOptions,clickedOption,afterValue:field.value};
};
export const editInteractionDate = async(root,arg)=>{
 const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
 function all(parent){
 const result=[],seen=new Set();
 function visit(el){for(const child of el.children||[]){
  if(seen.has(child))continue;
  seen.add(child);result.push(child);visit(child);
  if(child.shadowRoot)visit(child.shadowRoot);
 }}
 visit(parent);
 // The owned lightning-input-field is itself a shadow host.
 if(parent.shadowRoot)visit(parent.shadowRoot);
 return result;
}
 const fields=[...new Set(all(root).filter(node=>node.tagName==='LIGHTNING-INPUT-FIELD'))];
 if(fields.length!==1)throw new Error('L19_CONTROL_DATE_FIELD_NOT_UNIQUE');
 const field=fields[0];
 const inputs=[...new Set(all(field).filter(node=>node.tagName==='INPUT'&&!node.disabled&&!node.readOnly&&node.type!=='hidden'))];
 if(inputs.length!==1)throw new Error('L19_CONTROL_DATE_INPUT_NOT_UNIQUE');
 const input=inputs[0], inputText=arg.inputText;
 const beforeValue=field.value, beforeText=input.value;
 input.focus();
 Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,inputText);
 input.dispatchEvent(new Event('input',{bubbles:true,composed:true}));
 input.dispatchEvent(new Event('change',{bubbles:true,composed:true}));
 input.blur();await sleep(300);
 return {operation:arg.operation,inputText,beforeValue,beforeText,afterText:input.value,afterValue:field.value};
};
