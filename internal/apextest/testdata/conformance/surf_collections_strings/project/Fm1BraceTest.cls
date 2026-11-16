@IsTest
private class Fm1BraceTest {
 @IsTest static void observe() {
List<String> rows = new List<String>();
List<String> pats = new List<String>{
 'it\'s {0', 'a\\b {0', 'é {0', 'say "hi" {0', '日本 {0', 'line\nbreak {0', 'tab\there {0', '{', '{0}{', 'é\\"\'{0', 'plain {0'
};
for (Integer i = 0; i < pats.size(); i++) {
 try { rows.add(i + '=ok:' + String.format(pats[i], new List<Object>{'x'})); }
 catch (Exception e) { rows.add(i + '=' + e.getTypeName() + '|len=' + e.getMessage().length() + '|esc=' + e.getMessage().escapeUnicode()); }
}
 System.assert(false, 'OBS|' + String.join(rows, ';'));
 }
}
