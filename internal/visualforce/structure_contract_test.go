package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestStructureContractsRejectInvalidParentsAndCardinality(t *testing.T) {
	cases := []struct {
		name          string
		page          string
		component     string
		wantLoadError bool
	}{
		{
			name: "param under commandLink in form",
			page: `<apex:page><apex:form><apex:commandLink value="Open"><apex:param name="id" value="42"/></apex:commandLink></apex:form></apex:page>`,
		},
		{
			name: "param under existing outputFormat renderer",
			page: `<apex:page><apex:outputFormat value="Hello {0}"><apex:param value="Ada"/></apex:outputFormat></apex:page>`,
		},
		{
			// Native child_outputPanel_param accepts this parent at API 59/67.
			name: "param under outputPanel parent",
			page: `<apex:page><apex:outputPanel><apex:param name="id" value="42"/></apex:outputPanel></apex:page>`,
		},
		{
			name:          "multiple page roots",
			page:          `<apex:page/><apex:page/>`,
			wantLoadError: true,
		},
		{
			name:          "attribute declaration outside component",
			page:          `<apex:page><apex:attribute name="value" type="String" required="true"/></apex:page>`,
			wantLoadError: true,
		},
		{
			name:          "componentBody outside component",
			page:          `<apex:page><apex:componentBody/></apex:page>`,
			wantLoadError: true,
		},
		{
			name: "required attribute and one componentBody inside component",
			// Native diagnostic_description_missing requires the declaration description.
			component: `<apex:component><apex:attribute name="value" type="String" required="true" description="Body value"/><apex:repeat value="{!items}" var="item"><section><apex:componentBody/></section></apex:repeat></apex:component>`,
		},
		{
			name:          "multiple component roots",
			component:     `<apex:component/><apex:component/>`,
			wantLoadError: true,
		},
		{
			// Native diagnostic_body_duplicate accepts repeated direct slots.
			name:      "duplicate direct componentBody declarations are accepted",
			component: `<apex:component><apex:componentBody/><apex:componentBody/></apex:component>`,
		},
		{
			name:          "componentBody nested within componentBody",
			component:     `<apex:component><apex:componentBody><apex:componentBody/></apex:componentBody></apex:component>`,
			wantLoadError: true,
		},
		{
			name:          "duplicate nested componentBody declarations",
			component:     `<apex:component><apex:repeat value="{!items}" var="item"><section><apex:componentBody/></section></apex:repeat><div><apex:componentBody/></div></apex:component>`,
			wantLoadError: true,
		},
		{
			name: "pageBlockSectionItem accepts two children",
			// Native cardinality_sectionItem_in_section_2 needs section context.
			page: `<apex:page><apex:pageBlockSection><apex:pageBlockSectionItem><apex:outputText value="label"/><apex:outputText value="value"/></apex:pageBlockSectionItem></apex:pageBlockSection></apex:page>`,
		},
		{
			name: "pageBlockSectionItem rejects a third child",
			// Native cardinality_sectionItem_in_section_3 rejects the third child.
			page:          `<apex:page><apex:pageBlockSection><apex:pageBlockSectionItem><apex:outputText value="label"/><apex:outputText value="value"/><apex:outputText value="extra"/></apex:pageBlockSectionItem></apex:pageBlockSection></apex:page>`,
			wantLoadError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, index, err := loadStructureContractFixture(t, tc.page, tc.component)
			if tc.wantLoadError {
				if err == nil {
					t.Fatal("LoadProject accepted structurally invalid markup")
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadProject rejected normative positive case: %v", err)
			}
			if tc.page != "" {
				if _, ok := index.Page("Structure"); !ok {
					t.Fatal("LoadProject did not index the positive page")
				}
			}
			if tc.component != "" {
				component, ok := index.Component("Structure")
				if !ok {
					t.Fatal("LoadProject did not index the positive component")
				}
				if strings.Contains(tc.component, "<apex:attribute") && (len(component.Attributes) != 1 || component.Attributes[0].Required != "true") {
					t.Fatalf("LoadProject did not preserve the required component attribute declaration: %#v", component)
				}
			}
		})
	}
}

func TestStructureContractsRenderPageBlockSectionItemChildrenInOrder(t *testing.T) {
	page := `<apex:page><apex:pageBlock><apex:pageBlockSection><apex:pageBlockSectionItem><apex:outputText value="label-marker"/><apex:outputText value="value-marker"/></apex:pageBlockSectionItem></apex:pageBlockSection></apex:pageBlock></apex:page>`
	project, index, err := loadStructureContractFixture(t, page, "")
	if err != nil {
		t.Fatalf("LoadProject rejected two-child pageBlockSectionItem: %v", err)
	}
	result, err := RenderPage(PageRenderRequest{Project: project, VFIndex: index, PageName: "Structure"})
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	label := strings.Index(result.HTML, "label-marker")
	value := strings.Index(result.HTML, "value-marker")
	if label < 0 || value < 0 || label >= value {
		t.Fatalf("pageBlockSectionItem child document order not preserved: %s", result.HTML)
	}
}

func TestStructureValidationUsesHTMLRawScriptText(t *testing.T) {
	markup := `<apex:page><script>const example = "<apex:param name='not-a-child'/>";</script><apex:form/></apex:page>`
	if _, err := ParseMarkupTree(markup); err != nil {
		t.Fatalf("raw script text was treated as Visualforce structure: %v", err)
	}
}

func TestRenderPageRejectsStructureFromDirectIndex(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "DirectStructure.page")
	// Native order_column_outside_table is a structural rejection at both APIs.
	writeFile(t, path, `<apex:page><apex:pageBlock><apex:column headerValue="A"/></apex:pageBlock></apex:page>`)
	index := Index{Pages: []Page{{Name: "DirectStructure", File: path}}}
	index.sortAndBuildLookups()
	if _, err := RenderPage(PageRenderRequest{VFIndex: index, PageName: "DirectStructure"}); err == nil {
		t.Fatal("RenderPage bypassed shared markup structure validation")
	}
}

func TestLoadProjectBestEffortSkipsStructureValidation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	// Keep a rejected source (order_column_outside_table) on the lenient path.
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Lenient.page"), `<apex:page><apex:pageBlock><apex:column headerValue="A"/></apex:pageBlock></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index := LoadProjectBestEffort(p)
	if _, ok := index.Page("Lenient"); !ok {
		t.Fatal("LoadProjectBestEffort dropped a page solely for a structural-rule violation")
	}
}

func loadStructureContractFixture(t *testing.T, pageMarkup, componentMarkup string) (project.Project, Index, error) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	if pageMarkup != "" {
		path := filepath.Join(root, "force-app/main/default/pages/Structure.page")
		writeFile(t, path, pageMarkup)
		writeFile(t, path+"-meta.xml", `<ApexPage><apiVersion>67.0</apiVersion></ApexPage>`)
	}
	if componentMarkup != "" {
		path := filepath.Join(root, "force-app/main/default/components/Structure.component")
		writeFile(t, path, componentMarkup)
		writeFile(t, path+"-meta.xml", `<ApexComponent><apiVersion>67.0</apiVersion></ApexComponent>`)
	}
	p, err := project.Load(root)
	if err != nil {
		return project.Project{}, Index{}, err
	}
	index, err := LoadProject(p)
	return p, index, err
}
