package visualforce

import (
	"strconv"
	"strings"
)

type AjaxPayload struct {
	IsAjax          bool
	Action          string
	RerenderTargets []string
	SubmittedFields map[string]string
}

func ParseAjaxPayload(values map[string]string) AjaxPayload {
	payload := AjaxPayload{
		IsAjax:          strings.TrimSpace(values["__vf_ajax"]) == "1",
		Action:          strings.TrimSpace(values[ViewStateActionFieldName()]),
		RerenderTargets: ParseRerenderTargets(values["__vf_rerender"]),
		SubmittedFields: make(map[string]string),
	}
	for key, value := range values {
		if isAjaxControlField(key) {
			continue
		}
		payload.SubmittedFields[key] = value
	}
	return payload
}

func isAjaxControlField(key string) bool {
	switch key {
	case "__vf_ajax", "__vf_rerender", "__vf_csrf", viewStateActionField, viewStateFieldName:
		return true
	default:
		return false
	}
}

func VisualforceAjaxScript() string {
	return `<script>
window.GLADEVF=window.GLADEVF||{};
function appendValue(data,name,value){if(name){data.append(name,value==null?"":String(value));}}
function appendControl(data,el){
  if(!el||el.disabled||!el.name){return;}
  var tag=String(el.tagName||"").toLowerCase();
  var type=String(el.type||"").toLowerCase();
  if((type==="checkbox"||type==="radio")&&!el.checked){return;}
  if(tag==="select"&&el.multiple){Array.prototype.forEach.call(el.options||[],function(o){if(o.selected){appendValue(data,el.name,o.value);}});return;}
  appendValue(data,el.name,el.value);
}
function appendControls(data,root){
  if(!root||!root.querySelectorAll){return;}
  Array.prototype.forEach.call(root.querySelectorAll("input,select,textarea,button"),function(el){appendControl(data,el);});
}
function appendFormControlFields(data,form){
  ["` + ViewStateActionFieldName() + `","` + ViewStateFormFieldName() + `","__vf_csrf"].forEach(function(name){
    var el=form&&form.querySelector&&form.querySelector('[name="'+name+'"]');
    if(el){appendControl(data,el);}
  });
}
function appendParams(data,params){
  (params||[]).forEach(function(param){if(param&&param.name){var value=param.value==null?"":String(param.value);data.set(param.name,value);if(param.assignTo){data.set(param.assignTo,value);}}});
}
function statusRoot(id){
  if(!id||!document.querySelectorAll){return null;}
  var nodes=document.querySelectorAll("[data-status]");
  for(var i=0;i<nodes.length;i++){if(nodes[i].getAttribute("data-status")===String(id)){return nodes[i];}}
  return null;
}
function setStatus(id,active){
  var root=statusRoot(id); if(!root){return;}
  root.setAttribute("data-active",active?"true":"false");
  Array.prototype.forEach.call(root.querySelectorAll(".actionStatusStart"),function(el){el.hidden=!active;});
  Array.prototype.forEach.call(root.querySelectorAll(".actionStatusStop"),function(el){el.hidden=!!active;});
}
window.GLADEVF.submit=function(form,action,targets,options){
  if(!form){return false;}
  options=options||{};
  var data=new URLSearchParams();
  if(options.region){appendControls(data,options.region);appendFormControlFields(data,form);}else{appendControls(data,form);}
  appendParams(data,options.params);
  data.set("` + ViewStateActionFieldName() + `",action||"");
  data.set("__vf_ajax","1");
  data.set("__vf_rerender",targets||"");
  setStatus(options.status,true);
  fetch(form.action,{method:"POST",body:data})
    .then(function(r){return r.json();})
    .then(function(p){
      if(p.redirect){setStatus(options.status,false);window.location.assign(p.redirect);return;}
      Object.keys(p.targets||{}).forEach(function(id){
        var el=document.getElementById(id); if(el){el.outerHTML=p.targets[id];}
      });
      (p.messages||[]).forEach(function(m){if(window.console&&console.warn){console.warn(m);}});
      var vs=form.elements["` + ViewStateFormFieldName() + `"]; if(vs&&p.viewState){vs.value=p.viewState;}
      setStatus(options.status,false);
    },function(err){
      setStatus(options.status,false);
      throw err;
    });
  return false;
};
</script>`
}

func VisualforceAjaxSubmitHook(action, targets string) string {
	return VisualforceAjaxSubmitHookWithStatus(action, targets, "")
}

func VisualforceAjaxLinkHook(action, targets string) string {
	return VisualforceAjaxLinkHookWithStatus(action, targets, "")
}

func VisualforceAjaxSubmitHookWithStatus(action, targets, status string) string {
	options := jsAjaxOptionsLiteral(status, nil)
	return "var e=(typeof event!='undefined'&&event)||window.event;var f=(e&&e.currentTarget&&e.currentTarget.form)||document.forms[0];var o=" + options + ";var r=e&&e.currentTarget&&e.currentTarget.closest&&e.currentTarget.closest('[data-vf-region]');if(r){o.region=r;}return window.GLADEVF.submit(f," + jsStringLiteral(action) + "," + jsStringLiteral(targets) + ",o);"
}

func VisualforceAjaxLinkHookWithStatus(action, targets, status string) string {
	options := jsAjaxOptionsLiteral(status, nil)
	return "var f=this.closest('form')||document.forms[0];var o=" + options + ";var r=this.closest&&this.closest('[data-vf-region]');if(r){o.region=r;}return window.GLADEVF.submit(f," + jsStringLiteral(action) + "," + jsStringLiteral(targets) + ",o);"
}

type VisualforceAjaxParam struct {
	Name         string
	DefaultValue string
	ArgumentName string
	AssignTo     string
}

func visualforceAjaxParams(node *MarkupNode, ctx *RenderContext) ([]VisualforceAjaxParam, error) {
	if node == nil {
		return nil, nil
	}
	params := make([]VisualforceAjaxParam, 0)
	for _, child := range node.Children {
		if child == nil || child.Type != MarkupNodeElement || !strings.EqualFold(child.Namespace, "apex") || !strings.EqualFold(child.Name, "param") {
			continue
		}
		name := strings.TrimSpace(child.Attribute("name"))
		if name == "" {
			continue
		}
		value := strings.TrimSpace(child.Attribute("value"))
		if ctx != nil {
			rendered, err := RenderExpressionTemplate(value, ctx.Expression)
			if err != nil {
				return nil, err
			}
			value = strings.TrimSpace(rendered)
		}
		params = append(params, VisualforceAjaxParam{
			Name:         name,
			DefaultValue: value,
			AssignTo:     expressionFieldName(child.Attribute("assignTo")),
		})
	}
	return params, nil
}

func VisualforceAjaxFunctionArgs(params []VisualforceAjaxParam) string {
	names := make([]string, 0, len(params))
	for i, param := range params {
		names = append(names, jsParamArgumentName(param, i))
	}
	return strings.Join(names, ",")
}

func VisualforceAjaxFunctionCall(action, targets, status string, params []VisualforceAjaxParam) string {
	options := jsAjaxOptionsLiteral(status, params)
	return "var f=document.forms[0];return window.GLADEVF.submit(f," + jsStringLiteral(action) + "," + jsStringLiteral(targets) + "," + options + ");"
}

func jsAjaxOptionsLiteral(status string, params []VisualforceAjaxParam) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(status) != "" {
		parts = append(parts, "status:"+jsStringLiteral(status))
	}
	if len(params) > 0 {
		items := make([]string, 0, len(params))
		for i, param := range params {
			name := strings.TrimSpace(param.Name)
			if name == "" {
				continue
			}
			arg := jsParamArgumentName(param, i)
			value := arg
			if strings.TrimSpace(param.DefaultValue) != "" {
				value = "(" + arg + "!==undefined?" + arg + ":" + jsStringLiteral(param.DefaultValue) + ")"
			}
			item := "{name:" + jsStringLiteral(name) + ",value:" + value
			if strings.TrimSpace(param.AssignTo) != "" && !strings.EqualFold(strings.TrimSpace(param.AssignTo), name) {
				item += ",assignTo:" + jsStringLiteral(strings.TrimSpace(param.AssignTo))
			}
			item += "}"
			items = append(items, item)
		}
		if len(items) > 0 {
			parts = append(parts, "params:["+strings.Join(items, ",")+"]")
		}
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func jsParamArgumentName(param VisualforceAjaxParam, index int) string {
	if param.ArgumentName != "" {
		return param.ArgumentName
	}
	name := strings.TrimSpace(param.Name)
	if isSafeJSIdentifier(name) {
		return name
	}
	return "p" + strconv.Itoa(index)
}

func isSafeJSIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if i == 0 {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || ch == '$') {
				return false
			}
			continue
		}
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '$') {
			return false
		}
	}
	return true
}

func jsStringLiteral(raw string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\r", `\r`)
	return "'" + replacer.Replace(raw) + "'"
}
