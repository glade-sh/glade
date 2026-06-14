#!/usr/bin/env sh
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
repo="$(CDPATH= cd -- "$root/../../.." && pwd)"
vf="$repo/testdata/local-tests/lightning-out-vf"

need_file() {
    if [ ! -f "$root/$1" ]; then
        echo "missing $1" >&2
        exit 1
    fi
}

need_text() {
    file="$1"
    text="$2"
    if ! grep -Fq "$text" "$root/$file"; then
        echo "missing '$text' in $file" >&2
        exit 1
    fi
}

need_file "sfdx-project.json"
need_text "sfdx-project.json" "\"path\": \"force-app\""

need_file "force-app/main/default/classes/LwcProbeController.cls"
need_file "force-app/main/default/classes/LwcProbeController.cls-meta.xml"
need_text "force-app/main/default/classes/LwcProbeController.cls" "@AuraEnabled(cacheable=true)"
need_text "force-app/main/default/classes/LwcProbeController.cls" "wireAccounts"
need_text "force-app/main/default/classes/LwcProbeController.cls" "imperativeAccount"

for bundle in contextProbe recordProbe wireProbe; do
    need_file "force-app/main/default/lwc/$bundle/$bundle.js"
    need_file "force-app/main/default/lwc/$bundle/$bundle.html"
    need_file "force-app/main/default/lwc/$bundle/$bundle.js-meta.xml"
done

need_text "force-app/main/default/lwc/contextProbe/contextProbe.js" "@salesforce/label/c.LwcProbeGreeting"
need_text "force-app/main/default/lwc/contextProbe/contextProbe.js" "@salesforce/resourceUrl/LwcProbeAssets"
need_text "force-app/main/default/lwc/recordProbe/recordProbe.js" "lightning/uiRecordApi"
need_text "force-app/main/default/lwc/wireProbe/wireProbe.js" "@salesforce/apex/LwcProbeController.wireAccounts"
need_text "force-app/main/default/lwc/wireProbe/wireProbe.js" "imperativeAccount"

for page in Account_Record_Page Sales_Dashboard Custom_Home; do
    need_file "force-app/main/default/flexipages/$page.flexipage-meta.xml"
done

need_text "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml" "contextProbe"
need_text "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml" "recordProbe"
need_text "force-app/main/default/flexipages/Sales_Dashboard.flexipage-meta.xml" "wireProbe"
need_text "force-app/main/default/flexipages/Custom_Home.flexipage-meta.xml" "contextProbe"

need_file "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml"
need_text "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml" "<label>LWC Probe</label>"

need_file "force-app/main/default/labels/CustomLabels.labels"
need_file "force-app/main/default/staticresources/LwcProbeAssets.resource"
need_file "force-app/main/default/staticresources/LwcProbeAssets.resource-meta.xml"
need_file "data/accounts.json"
need_text "data/accounts.json" "Local Shell Account"

for bundle in contextProbe wireProbe; do
    if [ ! -f "$vf/force-app/main/default/lwc/$bundle/$bundle.js" ]; then
        echo "missing lightning-out-vf lwc/$bundle/$bundle.js" >&2
        exit 1
    fi
    if [ ! -f "$vf/force-app/main/default/lwc/$bundle/$bundle.html" ]; then
        echo "missing lightning-out-vf lwc/$bundle/$bundle.html" >&2
        exit 1
    fi
    if [ ! -f "$vf/force-app/main/default/lwc/$bundle/$bundle.js-meta.xml" ]; then
        echo "missing lightning-out-vf lwc/$bundle/$bundle.js-meta.xml" >&2
        exit 1
    fi
done

if ! grep -Fq 'mount("c:contextProbe"' "$vf/force-app/main/default/pages/MultiWidgetHost.page"; then
    echo "missing contextProbe mount in lightning-out-vf MultiWidgetHost.page" >&2
    exit 1
fi
if ! grep -Fq 'mount("c:wireProbe"' "$vf/force-app/main/default/pages/MultiWidgetHost.page"; then
    echo "missing wireProbe mount in lightning-out-vf MultiWidgetHost.page" >&2
    exit 1
fi
if ! grep -Fq 'mount("c:recordWireHost"' "$vf/force-app/main/default/pages/MultiWidgetHost.page"; then
    echo "missing record wire mount in lightning-out-vf MultiWidgetHost.page" >&2
    exit 1
fi

(cd "$repo" && go test ./internal/project ./internal/lwc -run 'LWC|Project' -count=1)
