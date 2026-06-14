import assert from "node:assert/strict";
import http from "node:http";
import test from "node:test";
import { chromium } from "playwright";

function startVisualforceHarness() {
  const seen = [];
  const server = http.createServer((req, res) => {
    const chunks = [];
    req.on("data", (chunk) => chunks.push(chunk));
    req.on("end", () => {
      const body = Buffer.concat(chunks).toString("utf8");
      if (req.url === "/apex/Remote/remoting") {
        seen.push({ url: req.url, body: JSON.parse(body) });
        res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
        res.end(JSON.stringify([{ action: "RemoteController", method: "inspect", type: "rpc", tid: 1, status: true, result: { ok: true } }]));
        return;
      }
      if (req.url === "/apex/Remote/remoteObjects") {
        seen.push({ url: req.url, body: JSON.parse(body) });
        res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
        if (body.includes('"operation":"describe"')) {
          res.end(JSON.stringify({ success: true, describe: { name: "Account", fields: [{ name: "Name", jsName: "Name" }] } }));
          return;
        }
        res.end(JSON.stringify({ success: true, records: [{ Id: "001000000000001", Name: "Acme" }] }));
        return;
      }
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(pageHTML());
    });
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      resolve({
        baseURL: `http://127.0.0.1:${port}`,
        seen,
        close: () => new Promise((done) => server.close(done)),
      });
    });
  });
}

function pageHTML() {
  return `<!DOCTYPE html>
<html>
<body>
  <input name="com.salesforce.visualforce.ViewState" value="view-state">
  <input name="__vf_csrf" value="csrf-token">
  <script>
    (function(window){
      window.Visualforce=window.Visualforce||{};
      Visualforce.remoting=Visualforce.remoting||{};
      Visualforce.remoting.Manager=Visualforce.remoting.Manager||{};
      Visualforce.remoting.Manager._tid=Visualforce.remoting.Manager._tid||1;
      Visualforce.remoting.Manager.invokeAction=function(remoteAction){
        var values=Array.prototype.slice.call(arguments,1);
        var callback=null;
        if(values.length&&typeof values[values.length-1]=="function"){callback=values.pop();}
        var read=function(name){var el=document.querySelector('input[name="'+name+'"]');return el?el.value:"";};
        var actionText=String(remoteAction||"");
        var actionName=actionText.replace(/^\\{!\\$RemoteAction\\./,"").replace(/\\}$/,"");
        var dot=actionName.lastIndexOf(".");
        var action=dot>=0?actionName.slice(0,dot):actionName;
        var method=dot>=0?actionName.slice(dot+1):"";
        var request={action:action,method:method,data:values,type:"rpc",tid:Visualforce.remoting.Manager._tid++,ctx:{page:window.location.pathname,viewState:read("com.salesforce.visualforce.ViewState"),csrf:read("__vf_csrf")}};
        return fetch(window.location.pathname.replace(/\\/$/,"")+"/remoting",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify([request])}).then(function(response){return response.json();}).then(function(responses){var response=Array.isArray(responses)?responses[0]:responses;var event={status:!!(response&&response.status),type:(response&&response.type)||"rpc",tid:response&&response.tid,action:response&&response.action,method:response&&response.method,message:response&&response.message,where:response&&response.where};if(callback){callback(response?response.result:null,event);}return response;});
      };
      window.RemoteController={inspect:function(){var args=Array.prototype.slice.call(arguments);args.unshift("{!$RemoteAction.RemoteController.inspect}");return Visualforce.remoting.Manager.invokeAction.apply(Visualforce.remoting.Manager,args);}};
    })(window);
  </script>
  <script>
    (function(window){
      window.__gladeRemoteObjects=function(operation,objectName,fields,callback,criteria){
        fields=fields||{};
        var read=function(name){var el=document.querySelector('input[name="'+name+'"]');return el?el.value:"";};
        return fetch(window.location.pathname.replace(/\\/$/,"")+"/remoteObjects",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({operation:operation,objectName:objectName,fields:fields,criteria:criteria||null,ids:[],viewState:read("com.salesforce.visualforce.ViewState"),csrf:read("__vf_csrf")})}).then(function(response){return response.json();}).then(function(result){if(callback){callback(result,{status:!!(result&&result.success),type:"remoteObjects"});}return result;});
      };
      var RemoteObjectModel=window.RemoteObjectModel||{};
      window.RemoteObjectModel=RemoteObjectModel;
      RemoteObjectModel.Account=function(fields){this.fields=fields||{};};
      RemoteObjectModel.Account.describe=function(callback){return window.__gladeRemoteObjects("describe","Account",{},callback);};
      RemoteObjectModel.Account.query=function(criteria,callback){if(typeof criteria=="function"){callback=criteria;criteria={};}return window.__gladeRemoteObjects("query","Account",{},callback,criteria||{});};
    })(window);
  </script>
</body>
</html>`;
}

test("Visualforce remoting and Remote Objects browser envelopes keep practical parameter shapes", async () => {
  const server = await startVisualforceHarness();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/apex/Remote`, { waitUntil: "networkidle" });
    const result = await page.evaluate(async () => {
      const remoting = await window.RemoteController.inspect({ name: "trail" }, ["one", "two"]);
      const describe = await window.RemoteObjectModel.Account.describe();
      const query = await window.RemoteObjectModel.Account.query({ limit: 10 });
      return { remoting, describe, query };
    });
    assert.equal(result.remoting.status, true);
    assert.equal(result.describe.describe.name, "Account");
    assert.equal(result.query.records[0].Name, "Acme");

    assert.deepEqual(server.seen[0].body[0].data, [{ name: "trail" }, ["one", "two"]]);
    assert.equal(server.seen[0].body[0].ctx.viewState, "view-state");
    assert.equal(server.seen[1].body.operation, "describe");
    assert.equal(server.seen[2].body.operation, "query");
    assert.deepEqual(server.seen[2].body.criteria, { limit: 10 });
  } finally {
    await browser.close();
    await server.close();
  }
});
