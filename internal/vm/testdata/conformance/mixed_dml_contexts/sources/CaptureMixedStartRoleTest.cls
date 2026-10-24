@IsTest private class CaptureMixedStartRoleTest {
    @IsTest static void observe() { insert new Account(Name='Before start'); Test.startTest(); String result; try { insert CaptureMixedDml.actor(true); result='success'; } catch(Exception e) { result=e.getTypeName()+': '+e.getMessage(); } Test.stopTest(); System.assert(false, 'OBS|'+JSON.serialize(result)); }
}
