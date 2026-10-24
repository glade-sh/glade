@IsTest private class EmailServicesTest {
    @IsTest static void active() { System.assert(false, 'CAPTURE|'+JSON.serialize(EmailServicesProbe.observe('active'))); }
    @IsTest static void inactive() { System.assert(false, 'CAPTURE|'+JSON.serialize(EmailServicesProbe.observe('inactive'))); }
    @IsTest static void missing() { System.assert(false, 'CAPTURE|'+JSON.serialize(EmailServicesProbe.observe('missing'))); }
    @IsTest static void limited() { System.assert(false, 'CAPTURE|'+JSON.serialize(EmailServicesProbe.observe('limited'))); }
}
