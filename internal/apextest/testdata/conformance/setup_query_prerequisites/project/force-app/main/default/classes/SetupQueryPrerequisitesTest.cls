@IsTest public class SetupQueryPrerequisitesTest {
    @IsTest static void metadata() {emit('metadata',SetupQueryPrerequisiteProbe.metadata());}
    @IsTest static void community() {emit('community',SetupQueryPrerequisiteProbe.community());}
    @IsTest static void packageLicenses() {emit('packageLicenses',SetupQueryPrerequisiteProbe.packageLicenses());}
    private static void emit(String method,Map<String,Object> row) {System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(row));}
}
