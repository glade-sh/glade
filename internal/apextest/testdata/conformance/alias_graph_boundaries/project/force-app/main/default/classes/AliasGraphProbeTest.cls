@IsTest private class AliasGraphProbeTest {
    @IsTest static void direct() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('direct')));}
    @IsTest static void knownChild() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('knownChild')));}
    @IsTest static void backReference() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('backReference')));}
    @IsTest static void cycle() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('cycle')));}
    @IsTest static void replacement() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('replacement')));}
    @IsTest static void property() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('property')));}
    @IsTest static void throwingProperty() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('throwingProperty')));}
    @IsTest static void setter() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('setter')));}
    @IsTest static void keyFailure() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('keyFailure')));}
    @IsTest static void coercion() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('coercion')));}
    @IsTest static void unqualifiedProperty() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('unqualifiedProperty')));}
    @IsTest static void externalProperty() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('externalProperty')));}
    @IsTest static void nestedProperty() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('nestedProperty')));}
    @IsTest static void staticProperty() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('staticProperty')));}
    @IsTest static void listProperty() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('listProperty')));}
    @IsTest static void setProperty() {System.assert(false,'OBS|'+JSON.serialize(AliasGraphProbe.observe('setProperty')));}
}
