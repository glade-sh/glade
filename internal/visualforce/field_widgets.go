package visualforce

// VisualforceFieldWidgetsScript implements the client-side state transition of
// compound field inputs. Date shortcuts update only the input value property;
// lookup edits/selection maintain the captured hidden field state.
// The native Choices controls keep their backing options
// in metadata order and change selected, while preserving defaultSelected. Both
// visible lists are unselected after a move; the named backing select supplies
// the values for ordinary and AJAX submissions.
func VisualforceFieldWidgetsScript() string {
	return `<script>
(function(){
  function move(id,select){
    var backing=document.getElementById(id);
    var available=document.getElementById(id+"_unselected");
    var chosen=document.getElementById(id+"_selected");
    if(!backing||!available||!chosen||backing.disabled){return false;}
    var source=select?available:chosen;
    var changes=Object.create(null);
    Array.prototype.forEach.call(source.options,function(option){
      if(option.selected&&!option.disabled){changes[option.value]=true;}
    });
    if(!Object.keys(changes).length){return false;}
    // Native arrows stabilize the first effective move at the widest integer
    // layout width plus a fixed 17px allowance. Keep it on subsequent moves:
    // the captured default/12px/24px fonts yield 90->107,103->120,182->199.
    if(!available.style.width&&!chosen.style.width){
      var width=Math.max(available.offsetWidth,chosen.offsetWidth);
      if(width>0){width+=17;available.style.width=width+"px";chosen.style.width=width+"px";}
    }
    // Visible option values are metadata indices. Combining both lists also
    // retains their mapping when inactive metadata options were omitted.
    var options=Array.prototype.slice.call(available.options).concat(Array.prototype.slice.call(chosen.options));
    options.sort(function(a,b){return Number(a.value)-Number(b.value);});
    var availableGroup=available.querySelector("optgroup")||available;
    var chosenGroup=chosen.querySelector("optgroup")||chosen;
    options.forEach(function(option,index){
      var value=backing.options[index];
      if(!value){return;}
      if(changes[option.value]){value.selected=select;}
      option.selected=false;
      (value.selected?chosenGroup:availableGroup).appendChild(option);
    });
    return false;
  }
  window.DatePicker=window.DatePicker||{};
  window.DatePicker.insertDate=function(value,id){
    var input=document.getElementById(id);
    if(input){input.value=value;}
    return false;
  };
  window.getElementByIdCS=function(id){return document.getElementById(id);};
  window.escapeUTF=function(value){return encodeURIComponent(value);};
  window.openLookup=function(path,width,unused,search){
    var target=new URL(path+(search||""),window.location.href);
    if(target.origin!==window.location.origin||target.pathname!=="/_ui/common/data/LookupPage"){return false;}
    window.open(target.href,"visualforceLookup","width="+width+",height=480,resizable=yes,scrollbars=yes");
    return false;
  };
  document.addEventListener("input",function(event){
    var input=event.target;
    if(!input||!input.id){return;}
    var id=document.getElementById(input.id+"_lkid");
    var previous=document.getElementById(input.id+"_lkold");
    var modified=document.getElementById(input.id+"_mod");
    if(id&&previous&&modified&&input.value!==previous.value){id.value="";modified.value="1";}
  });
  window.gladeVisualforceSelectLookup=function(formID,fieldID,id,name){
    var input=document.getElementById(fieldID);
    var selected=document.getElementById(fieldID+"_lkid");
    var previous=document.getElementById(fieldID+"_lkold");
    var prefix=document.getElementById(fieldID+"_lktp");
    var modified=document.getElementById(fieldID+"_mod");
    if(!input||!input.form||input.form.id!==formID||!selected||!previous||!prefix||!modified||id.substring(0,3)!==prefix.value){return false;}
    input.value=name;selected.value=id;previous.value=name;modified.value="1";
    return true;
  };
  window.MultiSelectPicklist=window.MultiSelectPicklist||{};
  window.MultiSelectPicklist.handleMSPSelect=function(id){return move(id,true);};
  window.MultiSelectPicklist.handleMSPUnSelect=function(id){return move(id,false);};
  window.Sfdc=window.Sfdc||{};
  if(typeof window.Sfdc.onload!=="function"){
    window.Sfdc.onload=function(callback){
      if(document.readyState==="loading"){document.addEventListener("DOMContentLoaded",callback,{once:true});}
      else{callback();}
    };
  }
  if(typeof window.blankshield!=="function"){
    window.blankshield=function(anchor){if(anchor){anchor.setAttribute("target","_blank");}};
  }
})();
</script>`
}
