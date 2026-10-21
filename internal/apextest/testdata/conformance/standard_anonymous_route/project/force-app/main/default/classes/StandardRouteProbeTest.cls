@IsTest private class StandardRouteProbeTest {
    @IsTest static void caseObject() { System.assert(false, 'OBSERVED|caseObject|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.caseObject()})); }
    @IsTest static void eventObject() { System.assert(false, 'OBSERVED|eventObject|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.eventObject()})); }
    @IsTest static void campaignObject() { System.assert(false, 'OBSERVED|campaignObject|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.campaignObject()})); }
    @IsTest static void orderObject() { System.assert(false, 'OBSERVED|orderObject|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.orderObject()})); }
    @IsTest static void contractObject() { System.assert(false, 'OBSERVED|contractObject|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.contractObject()})); }
    @IsTest static void assetObject() { System.assert(false, 'OBSERVED|assetObject|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.assetObject()})); }
    @IsTest static void accountControl() { System.assert(false, 'OBSERVED|accountControl|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.accountControl()})); }
    @IsTest static void contactControl() { System.assert(false, 'OBSERVED|contactControl|' + JSON.serialize(new Map<String,Object>{'value'=>StandardRouteProbe.contactControl()})); }
}
