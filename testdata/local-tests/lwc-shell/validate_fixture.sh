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
need_file "glade.lwc.json"
need_text "glade.lwc.json" "\"defaultContext\": \"accountRecord\""

for context in accountRecord salesDashboard home tab urlAddressableAction recordAction globalAction apexWire ldsRecord uiObjectInfo uiRelatedList uiLayout baseComponents packagePhase1BaseComponents phase3BaseComponents communityAccount; do
    need_text "glade.lwc.json" "\"$context\""
done

for target in recordPage appPage homePage tab urlAddressable recordAction globalAction component communityPage; do
    need_text "glade.lwc.json" "\"target\": \"$target\""
done
need_text "glade.lwc.json" "\"type\": \"comm__namedPage\""
need_text "glade.lwc.json" "\"basePath\": \"/partners\""

need_file "force-app/main/default/classes/LwcProbeController.cls"
need_file "force-app/main/default/classes/LwcProbeController.cls-meta.xml"
need_text "force-app/main/default/classes/LwcProbeController.cls" "@AuraEnabled(cacheable=true)"
need_text "force-app/main/default/classes/LwcProbeController.cls" "wireAccounts"
need_text "force-app/main/default/classes/LwcProbeController.cls" "imperativeAccount"

for bundle in contextProbe recordProbe wireProbe layoutProbe objectInfoProbe relatedListProbe baseComponentHost communityProbe communityThemeLayout; do
    need_file "force-app/main/default/lwc/$bundle/$bundle.js"
    need_file "force-app/main/default/lwc/$bundle/$bundle.html"
    need_file "force-app/main/default/lwc/$bundle/$bundle.js-meta.xml"
done

need_text "force-app/main/default/lwc/contextProbe/contextProbe.js" "@salesforce/label/c.LwcProbeGreeting"
need_text "force-app/main/default/lwc/contextProbe/contextProbe.js" "@salesforce/resourceUrl/LwcProbeAssets"
need_text "force-app/main/default/lwc/recordProbe/recordProbe.js" "lightning/uiRecordApi"
need_text "force-app/main/default/lwc/wireProbe/wireProbe.js" "@salesforce/apex/LwcProbeController.wireAccounts"
need_text "force-app/main/default/lwc/wireProbe/wireProbe.js" "imperativeAccount"
need_text "force-app/main/default/lwc/layoutProbe/layoutProbe.js" "lightning/uiLayoutApi"
need_text "force-app/main/default/lwc/objectInfoProbe/objectInfoProbe.js" "lightning/uiObjectInfoApi"
need_text "force-app/main/default/lwc/relatedListProbe/relatedListProbe.js" "lightning/uiRelatedListApi"
need_text "force-app/main/default/lwc/communityProbe/communityProbe.js" "@salesforce/community/basePath"
need_text "force-app/main/default/lwc/communityProbe/communityProbe.js" "@salesforce/community/Id"
need_text "force-app/main/default/lwc/communityProbe/communityProbe.js" "@salesforce/site/Id"
need_text "force-app/main/default/lwc/communityProbe/communityProbe.js" "comm__namedPage"
need_text "force-app/main/default/lwc/communityProbe/communityProbe.js-meta.xml" "lightningCommunity__Page"
need_text "force-app/main/default/lwc/communityProbe/communityProbe.js-meta.xml" "lightningCommunity__Default"
need_text "force-app/main/default/lwc/communityThemeLayout/communityThemeLayout.js-meta.xml" "lightningCommunity__Theme_Layout"

for tag in lightning-accordion lightning-accordion-section lightning-avatar lightning-badge lightning-breadcrumb lightning-breadcrumbs lightning-button-group lightning-button-icon-stateful lightning-button-menu lightning-button-stateful lightning-carousel lightning-carousel-image lightning-checkbox-group lightning-dual-listbox lightning-file-upload lightning-flow lightning-formatted-address lightning-formatted-date-time lightning-formatted-email lightning-formatted-number lightning-formatted-phone lightning-formatted-rich-text lightning-formatted-text lightning-formatted-time lightning-formatted-url lightning-helptext lightning-input-address lightning-input-rich-text lightning-map lightning-menu-divider lightning-menu-item lightning-menu-subheader lightning-pill lightning-pill-container lightning-progress-bar lightning-progress-indicator lightning-progress-ring lightning-progress-step lightning-quick-action-panel lightning-radio-group lightning-record-picker lightning-select lightning-slider lightning-tile lightning-tree lightning-tree-grid lightning-vertical-navigation lightning-vertical-navigation-item lightning-vertical-navigation-section; do
    need_text "force-app/main/default/lwc/baseComponentHost/baseComponentHost.html" "$tag"
done

for page in Account_Record_Page Sales_Dashboard Custom_Home; do
    need_file "force-app/main/default/flexipages/$page.flexipage-meta.xml"
done

need_text "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml" "contextProbe"
need_text "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml" "recordProbe"
need_text "force-app/main/default/flexipages/Account_Record_Page.flexipage-meta.xml" "itemInstances"
need_text "force-app/main/default/flexipages/Sales_Dashboard.flexipage-meta.xml" "wireProbe"
need_text "force-app/main/default/flexipages/Sales_Dashboard.flexipage-meta.xml" "contextProbe"
need_text "force-app/main/default/flexipages/Custom_Home.flexipage-meta.xml" "contextProbe"
need_text "force-app/main/default/flexipages/Custom_Home.flexipage-meta.xml" "c:lwcHomeTemplate"

need_file "force-app/main/default/aura/lwcHomeTemplate/lwcHomeTemplate.cmp"
need_file "force-app/main/default/aura/lwcHomeTemplate/lwcHomeTemplate.cmp-meta.xml"
need_file "force-app/main/default/aura/lwcHomeTemplate/lwcHomeTemplate.design"
need_text "force-app/main/default/aura/lwcHomeTemplate/lwcHomeTemplate.cmp" "lightning:homeTemplate"
need_file "force-app/main/default/aura/lightningOut/lightningOut.app"
need_text "force-app/main/default/aura/lightningOut/lightningOut.app" "c:contextProbe"
need_text "force-app/main/default/aura/lightningOut/lightningOut.app" "c:baseComponentHost"
need_file "force-app/main/default/pages/LwcShellProbe.page"
need_file "force-app/main/default/pages/MultiWidgetHost.page"
need_text "force-app/main/default/pages/LwcShellProbe.page" 'c:contextProbe'
need_text "force-app/main/default/pages/MultiWidgetHost.page" 'c:baseComponentHost'

need_file "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml"
need_text "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml" "<label>LWC Probe</label>"
need_text "force-app/main/default/tabs/Lwc_Probe.tab-meta.xml" "<flexiPage>Sales_Dashboard</flexiPage>"

need_file "force-app/main/default/labels/CustomLabels.labels"
need_file "force-app/main/default/staticresources/LwcProbeAssets.resource"
need_file "force-app/main/default/staticresources/LwcProbeAssets.resource-meta.xml"
need_file "data/accounts.json"
need_text "data/accounts.json" "Local Shell Account"

for bundle in contextProbe wireProbe objectInfoHost recordMutationHost serviceHost; do
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
if ! grep -Fq 'mount("c:objectInfoHost"' "$vf/force-app/main/default/pages/MultiWidgetHost.page"; then
    echo "missing object info mount in lightning-out-vf MultiWidgetHost.page" >&2
    exit 1
fi
if ! grep -Fq 'mount("c:recordMutationHost"' "$vf/force-app/main/default/pages/MultiWidgetHost.page"; then
    echo "missing record mutation mount in lightning-out-vf MultiWidgetHost.page" >&2
    exit 1
fi
if ! grep -Fq 'mount("c:serviceHost"' "$vf/force-app/main/default/pages/MultiWidgetHost.page"; then
    echo "missing service host mount in lightning-out-vf MultiWidgetHost.page" >&2
    exit 1
fi
if ! grep -Fq 'lightning/navigation' "$vf/force-app/main/default/lwc/serviceHost/serviceHost.js"; then
    echo "missing navigation service import in lightning-out-vf serviceHost" >&2
    exit 1
fi
if ! grep -Fq '@salesforce/messageChannel/LwcProbe__c' "$vf/force-app/main/default/lwc/serviceHost/serviceHost.js"; then
    echo "missing message channel import in lightning-out-vf serviceHost" >&2
    exit 1
fi
if [ ! -f "$vf/force-app/main/default/messageChannels/LwcProbe.messageChannel-meta.xml" ]; then
    echo "missing lightning-out-vf message channel metadata" >&2
    exit 1
fi
for resource in ServiceScript ServiceStyles; do
    if [ ! -f "$vf/force-app/main/default/staticresources/$resource.resource" ]; then
        echo "missing lightning-out-vf static resource $resource" >&2
        exit 1
    fi
    if [ ! -f "$vf/force-app/main/default/staticresources/$resource.resource-meta.xml" ]; then
        echo "missing lightning-out-vf static resource metadata $resource" >&2
        exit 1
    fi
done

(cd "$repo" && go test ./internal/project ./internal/lwc -run 'LWC|Project' -count=1)
