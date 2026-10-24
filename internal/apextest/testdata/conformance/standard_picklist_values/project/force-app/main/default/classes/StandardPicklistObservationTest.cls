@IsTest
private class StandardPicklistObservationTest {
    @IsTest static void opportunity() { System.assert(false,'PICKLIST|opportunity|'+StandardPicklistObservation.observe('opportunity')); }
    @IsTest static void lead() { System.assert(false,'PICKLIST|lead|'+StandardPicklistObservation.observe('lead')); }
    @IsTest static void account() { System.assert(false,'PICKLIST|account|'+StandardPicklistObservation.observe('account')); }
    @IsTest static void boundary() { System.assert(false,'PICKLIST|boundary|'+StandardPicklistObservation.observe('boundary')); }
    @IsTest static void rejection() { System.assert(false,'PICKLIST|rejection|'+StandardPicklistObservation.observe('rejection')); }
}
