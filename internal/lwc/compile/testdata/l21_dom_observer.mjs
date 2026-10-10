// Owned native L21 observer and control actions, exported from the native capture.
// Keep the observation surface identical; expected answers never enter the browser.
export const observeDOM = (host, additionalAttributes = []) => {
 const fixture=host.querySelector('[data-fixture]');
 const attrs=new Set(['class','role','title','name','type','value','disabled','checked','href','target','alt','tabindex','aria-label','aria-selected','aria-expanded','aria-hidden','aria-valuemin','aria-valuemax','aria-valuenow','aria-valuetext',...additionalAttributes]);
 function tree(node){
  if(node.nodeType===3){const text=node.textContent.replace(/\s+/g,' ').trim();return text?{text}:null;}
  if(node.nodeType!==1)return null;
  const tag=node.localName;const a={};
  for(const attr of node.attributes)if(attrs.has(attr.name))a[attr.name]=attr.value;
  const children=[...node.childNodes].map(tree).filter(Boolean);
  let shadow=null;if(node.shadowRoot)shadow=[...node.shadowRoot.childNodes].map(tree).filter(Boolean);
  return {tag,attrs:a,children,...(shadow?{shadow}:{})};
 }
 return fixture?{dom:tree(fixture)}:{noComponentWithinMs:500};
};

export const activateControl = (host, { step, includeTargetText = false }) => {
 const fixture=host.querySelector('[data-fixture]');const nodes=[];const visited=new Set();
 function walk(root){
  if(!root || visited.has(root))return;visited.add(root);
  for(const node of root.querySelectorAll('*')){
   if(node.matches(step[0]))nodes.push(node);
   if(node.shadowRoot)walk(node.shadowRoot);
  }
 }
 walk(fixture);const index=step[2]<0?nodes.length+step[2]:step[2];const target=nodes[index];
 if(!target)return {selector:step[0],operation:step[1],noControlWithinMs:500};
 try{target[step[1]]();return {selector:step[0],operation:step[1],targetTag:target.localName,...(includeTargetText?{targetText:(target.textContent || '').trim()}:{}),status:'returned'};}
 catch(error){return {selector:step[0],operation:step[1],thrown:{name:error.name,message:error.message}};}
};
