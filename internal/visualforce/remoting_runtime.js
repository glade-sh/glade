window.Visualforce=window.Visualforce||{};
Visualforce.remoting=Visualforce.remoting||{};
Visualforce.remoting.Manager=Visualforce.remoting.Manager||{};
Visualforce.remoting.Manager._tid=Visualforce.remoting.Manager._tid||1;
var exposed=Object.create(null);
(actions||[]).forEach(function(action){exposed[action.Action.toLowerCase()]=action;});
Visualforce.remoting.Manager.invokeAction=function(remoteAction){
    var values=Array.prototype.slice.call(arguments,1);
    var callback=null;
    var options={};
    if(values.length&&typeof values[values.length-1]=="function"){
        callback=values.pop();
    }else if(values.length>1&&typeof values[values.length-2]=="function"){
        options=values.pop()||{};
        callback=values.pop();
    }
    var actionText=String(remoteAction||"");
    var actionName=actionText.replace(/^\{!\$RemoteAction\./,"").replace(/\}$/,"");
    var descriptor=exposed[actionName.toLowerCase()];
    if(!descriptor){
        console.error("Unable to invoke action '"+actionName+"': no controller and/or function found");
        return;
    }
    if(!callback||values.length!==descriptor.ParameterCount){
        console.error("Visualforce Remoting: Parameter length does not match remote action parameters: expected "+descriptor.ParameterCount+" parameters, got ");
        return;
    }
    var read=function(name){var el=document.querySelector('input[name="'+name+'"]');return el?el.value:"";};
    var request={action:descriptor.ClassName,method:descriptor.MethodName,data:values,type:"rpc",tid:Visualforce.remoting.Manager._tid++,ctx:{page:window.location.pathname,viewState:read("__GLADE_VIEW_STATE_FIELD__"),csrf:read("__vf_csrf")}};
    var timeout=Number(options.timeout||30000);
    if(!Number.isFinite(timeout)){timeout=30000;}
    timeout=Math.max(0,Math.min(timeout,120000));
    var controller=new AbortController();
    var timedOut=false;
    var timer=setTimeout(function(){timedOut=true;controller.abort();},timeout);
    return fetch(window.location.pathname.replace(/\/$/,"")+"/remoting",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify([request]),signal:controller.signal}).then(function(response){
        return response.json();
    }).then(function(responses){
        clearTimeout(timer);
        var response=Array.isArray(responses)?responses[0]:responses;
        var event={status:!!(response&&response.status),type:(response&&response.type)||"rpc",tid:response&&response.tid,action:response&&response.action,method:response&&response.method};
        var result=response&&response.result!==undefined?response.result:null;
        if(!event.status){
            event.type="exception";
            event.message=response&&response.message;
            event.where=(response&&response.where)||"";
            result=null;
            console.error("Visualforce Remoting Exception: "+event.message,response);
        }else if(typeof result==="string"&&options.escape!==false&&options.escape!=="false"){
            result=result.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;");
        }
        callback(result,event);
        return response;
    }).catch(function(err){
        clearTimeout(timer);
        var message=timedOut?"Unable to connect to the server (transaction aborted: timeout).":String(err);
        var event={status:false,type:"exception",message:message};
        if(timedOut){console.error("Visualforce Remoting Exception: "+message,event);}
        callback(null,event);
        return {status:false,message:message,errors:[{message:message}]};
    });
};
