package vm

import (
	"encoding/json"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFeatureParameterDeclaredMetadataAnonymousRoute(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(false, System.FeatureManagement.checkPackageBooleanValue('LocalBooleanFeature'));
System.assertEquals(3, System.FeatureManagement.checkPackageIntegerValue('LocalIntegerFeature'));
System.assertEquals(Date.newInstance(2020, 1, 2), System.FeatureManagement.checkPackageDateValue('LocalDateFeature'));
System.FeatureManagement.setPackageBooleanValue('localbooleanfeature', true);
System.assertEquals(true, System.FeatureManagement.checkPackageBooleanValue('LOCALBOOLEANFEATURE'));
System.FeatureManagement.setPackageIntegerValue('localintegerfeature', 7);
System.assertEquals(7, System.FeatureManagement.checkPackageIntegerValue('LOCALINTEGERFEATURE'));
System.FeatureManagement.setPackageDateValue('localdatefeature', Date.newInstance(2026, 6, 3));
System.assertEquals(Date.newInstance(2026, 6, 3), System.FeatureManagement.checkPackageDateValue('LOCALDATEFEATURE'));
try {
    System.FeatureManagement.checkPackageDateValue('MissingLocalDateFeature');
    System.assert(false, 'expected missing feature parameter');
} catch (System.NoDataFoundException e) {
    System.assert('Unable to retrieve feature parameter MissingLocalDateFeature. No results found.'.equals(e.getMessage()));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	var org storage.OrgState
	if err := json.Unmarshal([]byte(`{"metadata":{"featureParameters":[
 {"name":"LocalBooleanFeature","type":"Boolean","dataflowDirection":"SubscriberToLmo","value":"false"},
 {"name":"LocalIntegerFeature","type":"Integer","dataflowDirection":"SubscriberToLmo","value":"3"},
 {"name":"LocalDateFeature","type":"Date","dataflowDirection":"SubscriberToLmo","value":"2020-01-02"}
 ]}}`), &org); err != nil {
		t.Fatal(err)
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
