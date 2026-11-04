@IsTest private class Case349ProbeTest {
 @IsTest static void checks() {
  System.assertEquals('DISTINCT=true,true,false;EQUAL=true,false,true;MULTI=true,true;REJECT=false,DUPLICATE_EXTERNAL_ID',Case349Probe.equalityProbe());
  System.assertEquals('B0=false,CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY;B1=false,CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY;B2=false,CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY;B3=EXCEPTION,System.SObjectException;B4=EXCEPTION,System.SObjectException;E0=true,false;E1=true,false;E2=false,INVALID_FIELD_FOR_INSERT_UPDATE;E3=false,INVALID_EMAIL_ADDRESS;E4=true,false;',Case349Probe.accountProbe());
 }
}
