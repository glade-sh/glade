package visualforce

import (
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
window.GLADEVF.submit=function(form,action,targets){
  if(!form){return false;}
  var data=new FormData(form);
  data.set("` + ViewStateActionFieldName() + `",action||"");
  data.set("__vf_ajax","1");
  data.set("__vf_rerender",targets||"");
  fetch(form.action,{method:"POST",body:new URLSearchParams(data)})
    .then(function(r){return r.json();})
    .then(function(p){
      Object.keys(p.targets||{}).forEach(function(id){
        var el=document.getElementById(id); if(el){el.outerHTML=p.targets[id];}
      });
      (p.messages||[]).forEach(function(m){if(window.console&&console.warn){console.warn(m);}});
      var vs=form.elements["` + ViewStateFormFieldName() + `"]; if(vs&&p.viewState){vs.value=p.viewState;}
    });
  return false;
};
</script>`
}

func VisualforceAjaxSubmitHook(action, targets string) string {
	return "var e=(typeof event!='undefined'&&event)||window.event;var f=(e&&e.currentTarget&&e.currentTarget.form)||document.forms[0];return window.GLADEVF.submit(f," + jsStringLiteral(action) + "," + jsStringLiteral(targets) + ");"
}

func VisualforceAjaxLinkHook(action, targets string) string {
	return "var f=this.closest('form')||document.forms[0];return window.GLADEVF.submit(f," + jsStringLiteral(action) + "," + jsStringLiteral(targets) + ");"
}

func jsStringLiteral(raw string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\r", `\r`)
	return "'" + replacer.Replace(raw) + "'"
}
