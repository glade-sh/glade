@IsTest
private class Rxd210067 {
    @IsTest static void d21_name_nel() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{NEXT LINE (NEL)}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_c1sel() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{START OF SELECTED AREA}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_c1apc() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{APPLICATION PROGRAM COMMAND}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_bell() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{BELL}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_escape() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{ESCAPE}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_cr() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{CARRIAGE RETURN (CR)}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_is1() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{INFORMATION SEPARATOR ONE}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_tab() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{CHARACTER TABULATION}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_ff() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{FORM FEED (FF)}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_latin1_80() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{LATIN 1 SUPPLEMENT 80}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_pua() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{PRIVATE USE AREA E000}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_cjk_lower() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{cjk unified ideographs 4e00}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_cjk_zero() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPHS 04E00}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_cjk_extb() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPHS EXTENSION B 20000}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_cjk_exta() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPHS EXTENSION A 3400}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_grinning_trim() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{  grinning face }'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_unassigned_block() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPHS A000}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_basic_hex_name() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{BASIC LATIN 7F}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_tangut() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{TANGUT 17000}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_unnamed_c1() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{LATIN 1 SUPPLEMENT 81}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_hex_plus() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{CJK UNIFIED IDEOGRAPHS +4E00}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
    @IsTest static void d21_name_emoji_block() {
        String observation;
        try { Pattern p = Pattern.compile('\\N{EMOTICONS 1F600}'); observation = 'ok'; } catch (Exception e) { observation = 'EXC ' + e.getTypeName() + '|' + (e instanceof StringException) + '|' + String.valueOf(e.getMessage()).replace('\n','\\n'); }
        System.assertEquals('ROW', observation);
    }
}
