@IsTest private class LocaleRuleProbeTest {
    @IsTest static void describeStatic() { System.assert(false, 'OBSERVED|describeStatic|' + JSON.serialize(new Map<String,Object>{'value'=>LocaleRuleProbe.describeStatic()})); }
    @IsTest static void describeDynamic() { System.assert(false, 'OBSERVED|describeDynamic|' + JSON.serialize(new Map<String,Object>{'value'=>LocaleRuleProbe.describeDynamic()})); }
    @IsTest static void queryLanguage() { System.assert(false, 'OBSERVED|queryLanguage|' + JSON.serialize(new Map<String,Object>{'value'=>LocaleRuleProbe.queryLanguage()})); }
    @IsTest static void instanceLanguage() { System.assert(false, 'OBSERVED|instanceLanguage|' + JSON.serialize(new Map<String,Object>{'value'=>LocaleRuleProbe.instanceLanguage()})); }
    @IsTest static void allLanguage() { System.assert(false, 'OBSERVED|allLanguage|' + JSON.serialize(new Map<String,Object>{'value'=>LocaleRuleProbe.allLanguage()})); }
    @IsTest static void constructedLanguage() { System.assert(false, 'OBSERVED|constructedLanguage|' + JSON.serialize(new Map<String,Object>{'value'=>LocaleRuleProbe.constructedLanguage()})); }
}
