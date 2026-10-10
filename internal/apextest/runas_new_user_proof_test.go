package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Seven unchanged API53 methods passed Salesforce waves103 and107.
func TestRunAsUserLifecycleContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRunAsLifecycle53.cls"), "@IsTest private class GladeRunAsLifecycle53 {\n static User fresh(){return new User(Username='glade103.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='probe',Email='probe@example.invalid',LastName='Glade103',ProfileId=UserInfo.getProfileId(),TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');}\n @IsTest static void persistsNewUser(){User u=fresh();System.assertEquals(null,u.Id);System.runAs(u){System.assertEquals(u.Id,UserInfo.getUserId());}System.assertNotEquals(null,u.Id);System.assertEquals(1,[SELECT COUNT() FROM User WHERE Id=:u.Id]);}\n @IsTest static void defaultsActive(){User u=fresh();System.runAs(u){} User row=[SELECT IsActive,Profile.Name FROM User WHERE Id=:u.Id];System.assertEquals(true,row.IsActive);System.assertEquals(1,[SELECT COUNT() FROM User WHERE Id=:u.Id AND IsActive=true]);System.assertNotEquals(null,row.Profile.Name);}\n @IsTest static void preservesExplicitInactive(){User u=fresh();u.IsActive=false;Boolean caught=false;try{System.runAs(u){}}catch(System.TypeException e){caught=true;System.assertEquals('System.runAs can only be used with an active user',e.getMessage());}System.assert(caught,'inactive user must be rejected');}\n @IsTest static void activeIdOnly(){User u=fresh();insert u;System.runAs(new User(Id=u.Id)){System.assertEquals(u.Id,UserInfo.getUserId());}}\n @IsTest static void inactiveIdOnly(){User u=fresh();u.IsActive=false;insert u;Boolean caught=false;try{System.runAs(new User(Id=u.Id)){}}catch(System.TypeException e){caught=true;System.assertEquals('System.runAs can only be used with an active user',e.getMessage());}System.assert(caught,'stored inactive user rejected');}\n @IsTest static void activeWithUnsavedInactive(){User u=fresh();insert u;u.IsActive=false;System.runAs(u){System.assertEquals(u.Id,UserInfo.getUserId());}System.assertEquals(true,[SELECT IsActive FROM User WHERE Id=:u.Id].IsActive,'unsaved values must not update stored user');System.assertEquals(false,u.IsActive,'caller value unchanged');}\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRunAsLifecycle53.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRunAsSetup53.cls"), "@IsTest private class GladeRunAsSetup53 {\n @TestSetup static void setupUser(){User u=new User(Username='glade103setup.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='probe',Email='probe@example.invalid',LastName='Glade103Setup',ProfileId=UserInfo.getProfileId(),TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');System.runAs(u){}}\n @IsTest static void setupUserRemainsActive(){System.assertEquals(1,[SELECT COUNT() FROM User WHERE LastName='Glade103Setup' AND IsActive=true]);}\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRunAsSetup53.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"53.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 7 || got.Passed != 7 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("runAs lifecycle: %s", data)
	}
}
