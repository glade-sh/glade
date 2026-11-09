import {createElement} from 'lwc';
import Dial from '/shim/clickToDial.js'; import Empty from '/shim/emptyState.js';
import Button from '/shim/richTextToolbarButton.js'; import Group from '/shim/richTextToolbarButtonGroup.js';
function add(tag,Ctor,props={}) {const el=createElement(tag,{is:Ctor});Object.assign(el,props);document.body.appendChild(el);return el;}
if(location.search==='?dial') for(const value of ['4155550100',undefined,'2125550123','abc']) add('lightning-click-to-dial',Dial,value===undefined?{}:{value});
if(location.search==='?empty'){ let empty=add('lightning-empty-state',Empty,{title:'No items'});let p=document.createElement('p');p.slot='description';p.textContent='Nothing here';empty.appendChild(p);add('lightning-empty-state',Empty); }
if(location.search==='?button'){ add('lightning-rich-text-toolbar-button',Button,{iconName:'utility:bold',iconAlternativeText:'Bold'});add('lightning-rich-text-toolbar-button',Button,{disabled:true});add('lightning-rich-text-toolbar-button',Button,{selected:true,iconAlternativeText:'Select'}); }
if(location.search==='?group'){ const group=add('lightning-rich-text-toolbar-button-group',Group,{ariaLabel:'Tools'});let b=createElement('lightning-rich-text-toolbar-button',{is:Button});Object.assign(b,{iconName:'utility:bold',iconAlternativeText:'Bold'});group.appendChild(b);add('lightning-rich-text-toolbar-button-group',Group); }
if(location.search==='?extra') {
 add('lightning-empty-state',Empty,{title:'Items',headingLevel:'2',illustrationName:'cart:noitems',alternativeText:'Empty cart',size:'small'});
 add('lightning-empty-state',Empty,{title:'Other',headingLevel:'9',size:'x-small'});
 const button=add('lightning-rich-text-toolbar-button',Button,{iconName:'utility:bold',iconAlternativeText:'Bold'});
 const content=document.createElement('span');content.textContent='Popup content';button.appendChild(content);window.__popupEvents=[];button.addEventListener('popupclickout',e=>{window.__popupEvents.push(e.type);if(window.__preventClickout)e.preventDefault()});const outside=document.createElement('button');outside.id='outside';outside.textContent='Outside';document.body.appendChild(outside);
}
requestAnimationFrame(()=>{window.__ready=true});
