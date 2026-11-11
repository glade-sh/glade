public class FernCases62 {
    public static String upd() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { Fern_Upd__c r = new Fern_Upd__c(Name = 'u1', Stage__c = 'a', Old__c = 'x'); insert r; out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Upd__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Upd__c r = [SELECT Id FROM Fern_Upd__c LIMIT 1]; r.Stage__c = 'b'; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Upd__c  ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Upd__c r = [SELECT Id FROM Fern_Upd__c LIMIT 1]; update r; out.add('again: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Upd__c  ORDER BY Name])); } catch (Exception e) { out.add('again: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { delete [SELECT Id FROM Fern_Upd__c]; insert new List<Fern_Upd__c>{ new Fern_Upd__c(Name = 'b1', Stage__c = 'p'), new Fern_Upd__c(Name = 'b2', Stage__c = 'q'), new Fern_Upd__c(Name = 'b3', Stage__c = 'r') }; out.add('bulk: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Upd__c  ORDER BY Name])); } catch (Exception e) { out.add('bulk: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String two() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Two__c(Name = 't1', Stage__c = 'a'); out.add('first: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Two__c WHERE Name='t1' ORDER BY Name])); } catch (Exception e) { out.add('first: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Two__c(Name = 't2', Stage__c = 'b', Flag__c = true); out.add('second: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Two__c WHERE Name='t2' ORDER BY Name])); } catch (Exception e) { out.add('second: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Two__c(Name = 't3', Stage__c = 'a', Flag__c = true); out.add('both: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Two__c WHERE Name='t3' ORDER BY Name])); } catch (Exception e) { out.add('both: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Two__c(Name = 't4', Stage__c = 'c', Flag__c = true); out.add('neither: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Two__c WHERE Name='t4' ORDER BY Name])); } catch (Exception e) { out.add('neither: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Two__c>{ new Fern_Two__c(Name = 't5', Stage__c = 'a'), new Fern_Two__c(Name = 't6', Stage__c = 'b', Flag__c = true), new Fern_Two__c(Name = 't7', Stage__c = 'c') }; out.add('mixed: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Two__c WHERE Name IN ('t5','t6','t7') ORDER BY Name])); } catch (Exception e) { out.add('mixed: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String twoorder() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Two__c>{ new Fern_Two__c(Name = 'o1', Stage__c = 'b', Flag__c = true), new Fern_Two__c(Name = 'o2', Stage__c = 'a'), new Fern_Two__c(Name = 'o3', Stage__c = 'b', Flag__c = true), new Fern_Two__c(Name = 'o4', Stage__c = 'a', Flag__c = true) }; out.add('rev: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Two__c WHERE Name LIKE 'o%' ORDER BY Name])); } catch (Exception e) { out.add('rev: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String evalbulk() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Next__c>{ new Fern_Next__c(Name = 'k1', Stage__c = 'a'), new Fern_Next__c(Name = 'k2', Stage__c = 'b', Note__c = 'R1'), new Fern_Next__c(Name = 'k3', Stage__c = 'c', Flag__c = true), new Fern_Next__c(Name = 'k4', Stage__c = 'a', Flag__c = true) }; out.add('mix: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Next__c WHERE Name LIKE 'k%' ORDER BY Name])); } catch (Exception e) { out.add('mix: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String seq() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Seq__c(Name = 's1', Stage__c = 'a'); out.add('hit: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Seq__c WHERE Name='s1' ORDER BY Name])); } catch (Exception e) { out.add('hit: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Seq__c(Name = 's2', Stage__c = 'b'); out.add('miss: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Seq__c WHERE Name='s2' ORDER BY Name])); } catch (Exception e) { out.add('miss: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Seq__c r = [SELECT Id FROM Fern_Seq__c WHERE Name='s2']; r.Stage__c = 'a'; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Seq__c WHERE Name='s2' ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String ordbulk() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Ord__c>{ new Fern_Ord__c(Name = 'z1', Stage__c = 'a'), new Fern_Ord__c(Name = 'z2', Stage__c = 'b'), new Fern_Ord__c(Name = 'z3', Stage__c = 'c') }; out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c WHERE Name LIKE 'z%' ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String bmp() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Bmp__c(Name = 'b1', Stage__c = 'a', Score__c = 0); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Bmp__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Bmp__c r = [SELECT Id FROM Fern_Bmp__c LIMIT 1]; r.Note__c = 'n'; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Bmp__c  ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String sib() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Sib__c(Name = 's1', Stage__c = 'target'); insert new Fern_Sib__c(Name = 's2', Stage__c = 'trig'); out.add('other: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Sib__c  ORDER BY Name])); } catch (Exception e) { out.add('other: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { delete [SELECT Id FROM Fern_Sib__c]; insert new List<Fern_Sib__c>{ new Fern_Sib__c(Name = 's3', Stage__c = 'target'), new Fern_Sib__c(Name = 's4', Stage__c = 'trig') }; out.add('cohort: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Sib__c  ORDER BY Name])); } catch (Exception e) { out.add('cohort: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String evalnext() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Next__c(Name = 'n1', Stage__c = 'a'); out.add('stale: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Next__c WHERE Name='n1' ORDER BY Name])); } catch (Exception e) { out.add('stale: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Next__c(Name = 'n2', Stage__c = 'a', Note__c = 'R1'); out.add('preset: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Next__c WHERE Name='n2' ORDER BY Name])); } catch (Exception e) { out.add('preset: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Next__c(Name = 'n3', Stage__c = 'b', Note__c = 'R1'); out.add('skipfirst: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Next__c WHERE Name='n3' ORDER BY Name])); } catch (Exception e) { out.add('skipfirst: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Next__c(Name = 'n4', Stage__c = 'a', Note__c = 'R1', Flag__c = true); out.add('all: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Next__c WHERE Name='n4' ORDER BY Name])); } catch (Exception e) { out.add('all: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Next__c(Name = 'n5', Stage__c = 'c', Flag__c = true); out.add('lastonly: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Next__c WHERE Name='n5' ORDER BY Name])); } catch (Exception e) { out.add('lastonly: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String grp() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Group__c(Name = 'g1', Stage__c = 'a'); out.add('one: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Group__c WHERE Name='g1' ORDER BY Name])); } catch (Exception e) { out.add('one: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Group__c>{ new Fern_Group__c(Name = 'g2'), new Fern_Group__c(Name = 'g3'), new Fern_Group__c(Name = 'g4') }; out.add('three: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Group__c WHERE Name IN ('g2','g3','g4') ORDER BY Name])); } catch (Exception e) { out.add('three: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String asg() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Asg__c(Name = 'a1', Stage__c = 'a'); out.add('match: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asg__c WHERE Name='a1' ORDER BY Name])); } catch (Exception e) { out.add('match: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Asg__c(Name = 'a2', Stage__c = 'b'); out.add('miss: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asg__c WHERE Name='a2' ORDER BY Name])); } catch (Exception e) { out.add('miss: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Asg__c r = [SELECT Id FROM Fern_Asg__c WHERE Name='a2']; r.Stage__c = 'a'; update r; out.add('later: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asg__c WHERE Name='a2' ORDER BY Name])); } catch (Exception e) { out.add('later: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String chg() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Chg__c(Name = 'c1', Stage__c = 'a', Flag__c = true); out.add('insflag: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('insflag: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Chg__c r = [SELECT Id FROM Fern_Chg__c WHERE Name='c1']; r.Stage__c = 'b'; r.Note__c = null; update r; out.add('changed: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('changed: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Chg__c r = [SELECT Id FROM Fern_Chg__c WHERE Name='c1']; r.Note__c = null; update r; out.add('unchanged: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('unchanged: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Chg__c r = [SELECT Id FROM Fern_Chg__c WHERE Name='c1']; r.Score__c = 10; update r; out.add('cross: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('cross: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Chg__c r = [SELECT Id FROM Fern_Chg__c WHERE Name='c1']; r.Old__c = null; r.Score__c = 11; update r; out.add('metagain: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('metagain: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Chg__c r = [SELECT Id FROM Fern_Chg__c WHERE Name='c1']; r.Score__c = 1; update r; out.add('backdown: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('backdown: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Chg__c r = [SELECT Id FROM Fern_Chg__c WHERE Name='c1']; r.Score__c = 12; update r; out.add('recross: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('recross: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Chg__c(Name = 'c2', Score__c = 20); out.add('insmet: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Chg__c WHERE Name='c2' ORDER BY Name])); } catch (Exception e) { out.add('insmet: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String met() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Met62__c(Name = 'm'); out.add('m62: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Met62__c  ORDER BY Name])); } catch (Exception e) { out.add('m62: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Met67__c(Name = 'm'); out.add('m67: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Met67__c  ORDER BY Name])); } catch (Exception e) { out.add('m67: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String rel() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { Fern_Root__c p = new Fern_Root__c(Name = 'p1'); insert p; insert new Fern_Rel__c(Name = 'r1', Stage__c = 'up', Root__c = p.Id); out.add('up: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Root__c WHERE Name='p1' ORDER BY Name]) + ' / ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rel__c WHERE Name='r1' ORDER BY Name])); } catch (Exception e) { out.add('up: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Rel__c(Name = 'r2', Stage__c = 'up'); out.add('nullparent: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rel__c WHERE Name='r2' ORDER BY Name])); } catch (Exception e) { out.add('nullparent: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Rel__c(Name = 'r3', Flag__c = true, Old__c = 'stay'); out.add('filtermiss: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rel__c WHERE Name='r3' ORDER BY Name])); } catch (Exception e) { out.add('filtermiss: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Rel__c(Name = 'r4', Flag__c = true, Old__c = 'go'); out.add('filterhit: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rel__c WHERE Name='r4' ORDER BY Name])); } catch (Exception e) { out.add('filterhit: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Root__c p = [SELECT Id FROM Fern_Root__c WHERE Name='p1']; p.Stage__c = 'cascade'; update p; out.add('children: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Root__c WHERE Name='p1' ORDER BY Name]) + ' / ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rel__c WHERE Name='r1' ORDER BY Name])); } catch (Exception e) { out.add('children: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String ord() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Ord__c(Name = 'o1', Stage__c = 'a'); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Ord__c r = [SELECT Id FROM Fern_Ord__c LIMIT 1]; r.Stage__c = 'b'; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c  ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String created() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_New__c(Name = 'w1'); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_New__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_New__c r = [SELECT Id FROM Fern_New__c LIMIT 1]; r.Note__c = 'touched'; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_New__c  ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String pri() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Pri__c(Name = 'q1', Stage__c = 'p'); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pri__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Pri__c r = [SELECT Id FROM Fern_Pri__c LIMIT 1]; r.Note__c = null; r.Stage__c = 'z'; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pri__c  ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Pri__c r = [SELECT Id FROM Fern_Pri__c LIMIT 1]; r.Old__c = null; update r; out.add('upd2: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pri__c  ORDER BY Name])); } catch (Exception e) { out.add('upd2: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String rec() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Rec__c(Name = 'e1', Stage__c = 'a', Score__c = 0); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rec__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String err() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Err__c(Name = 'x1', Stage__c = 'ok'); out.add('good: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Err__c  ORDER BY Name])); } catch (Exception e) { out.add('good: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Err__c(Name = 'x2', Stage__c = 'bad'); out.add('bad: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Err__c WHERE Name='x2' ORDER BY Name])); } catch (Exception e) { out.add('bad: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Err__c>{ new Fern_Err__c(Name = 'x3', Stage__c = 'ok'), new Fern_Err__c(Name = 'x4', Stage__c = 'bad') }, false); for (Database.SaveResult s : rs) { out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); } out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Err__c WHERE Name IN ('x3','x4') ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String flowact() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Flow__c(Name = 'f1'); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Flow__c WHERE Name='f1' ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Flow__c>{ new Fern_Flow__c(Name = 'f2'), new Fern_Flow__c(Name = 'f3'), new Fern_Flow__c(Name = 'f4') }; out.add('bulk: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Flow__c WHERE Name IN ('f2','f3','f4') ORDER BY Name])); } catch (Exception e) { out.add('bulk: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String thr() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Thr__c(Name = 'h1', Stage__c = 'ok'); out.add('ok: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Thr__c  ORDER BY Name])); } catch (Exception e) { out.add('ok: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Thr__c(Name = 'h2', Stage__c = 'bad'); out.add('bad: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Thr__c WHERE Name='h2' ORDER BY Name])); } catch (Exception e) { out.add('bad: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Thr__c>{ new Fern_Thr__c(Name = 'h3', Stage__c = 'ok'), new Fern_Thr__c(Name = 'h4', Stage__c = 'bad') }, false); for (Database.SaveResult s : rs) { out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); } out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Thr__c WHERE Name IN ('h3','h4') ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String cgl() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Cgl__c(Name = 'c1', Stage__c = 'a', Flag__c = true); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Cgl__c r = [SELECT Id FROM Fern_Cgl__c WHERE Name='c1']; r.Stage__c = 'b'; update r; out.add('changed: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c1' ORDER BY Name])); } catch (Exception e) { out.add('changed: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Cgl__c(Name = 'c2', Score__c = 1, Flag__c = true); out.add('score: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c2' ORDER BY Name])); } catch (Exception e) { out.add('score: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Cgl__c(Name = 'c3', Score__c = 1); out.add('noflag: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c3' ORDER BY Name])); } catch (Exception e) { out.add('noflag: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Cgl__c(Name = 'c4', Stage__c = 'x', Note__c = 'x'); out.add('fieldeq: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c4' ORDER BY Name])); } catch (Exception e) { out.add('fieldeq: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Cgl__c(Name = 'c5'); out.add('nullfield: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c5' ORDER BY Name])); } catch (Exception e) { out.add('nullfield: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Cgl__c(Name = 'c6', Day__c = Date.newInstance(2020,1,2)); out.add('dayafter: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c6' ORDER BY Name])); } catch (Exception e) { out.add('dayafter: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Cgl__c(Name = 'c7', Day__c = Date.newInstance(2020,1,1)); out.add('dayon: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Cgl__c WHERE Name='c7' ORDER BY Name])); } catch (Exception e) { out.add('dayon: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String fan() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Fan__c>{ new Fern_Fan__c(Name = 'a1', Stage__c = 'ok'), new Fern_Fan__c(Name = 'a2', Stage__c = 'bad'), new Fern_Fan__c(Name = 'a3', Stage__c = 'ok') }; out.add('aon: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fan__c  ORDER BY Name])); } catch (Exception e) { out.add('aon: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Fan__c>{ new Fern_Fan__c(Name = 'b1', Stage__c = 'bad'), new Fern_Fan__c(Name = 'b2', Stage__c = 'ok'), new Fern_Fan__c(Name = 'b3', Stage__c = 'bad') }; out.add('aon2: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fan__c  ORDER BY Name])); } catch (Exception e) { out.add('aon2: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Fan__c>{ new Fern_Fan__c(Name = 'c1', Stage__c = 'ok'), new Fern_Fan__c(Name = 'c2', Stage__c = 'bad'), new Fern_Fan__c(Name = 'c3', Stage__c = 'ok') }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fan__c  ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Fan__c>{ new Fern_Fan__c(Name = 'd1', Stage__c = 'bad'), new Fern_Fan__c(Name = 'd2', Stage__c = 'bad') }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('allbad: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fan__c WHERE Name LIKE 'd%' ORDER BY Name])); } catch (Exception e) { out.add('allbad: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { List<Fern_Fan__c> rows = [SELECT Id FROM Fern_Fan__c WHERE Name IN ('c1','c3') ORDER BY Name]; rows[0].Stage__c = 'bad'; rows[1].Stage__c = 'ok2'; Database.SaveResult[] rs = Database.update(rows, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('partupd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fan__c  ORDER BY Name])); } catch (Exception e) { out.add('partupd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String thrbulk() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Thr__c>{ new Fern_Thr__c(Name = 'k1', Stage__c = 'ok'), new Fern_Thr__c(Name = 'k2', Stage__c = 'bad'), new Fern_Thr__c(Name = 'k3', Stage__c = 'bad') }; out.add('aon: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Thr__c  ORDER BY Name])); } catch (Exception e) { out.add('aon: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Thr__c>{ new Fern_Thr__c(Name = 'k4', Stage__c = 'ok'), new Fern_Thr__c(Name = 'k5', Stage__c = 'bad'), new Fern_Thr__c(Name = 'k6', Stage__c = 'bad') }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Thr__c  ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String mergep() {
        List<String> out = new List<String>{ 'api=62.0' };
        Fern_Pa__c p1 = new Fern_Pa__c(Name = 'p1'); Fern_Pa__c p2 = new Fern_Pa__c(Name = 'p2'); insert new List<Fern_Pa__c>{ p1, p2 }; FernLog.take();
        try { insert new List<Fern_Prt__c>{ new Fern_Prt__c(Name = 'm1', Par__c = p1.Id, Flag__c = true, Stage__c = 'v', Score__c = 1), new Fern_Prt__c(Name = 'm2', Par__c = p1.Id, Flag__c = true, Stage__c = 'v', Score__c = 1) }; out.add('same: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pa__c  ORDER BY Name])); } catch (Exception e) { out.add('same: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Prt__c>{ new Fern_Prt__c(Name = 'm3', Par__c = p1.Id, Flag__c = true, Stage__c = 'x', Score__c = 2), new Fern_Prt__c(Name = 'm4', Par__c = p1.Id, Flag__c = true, Stage__c = 'y', Score__c = 3) }; out.add('differ: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pa__c  ORDER BY Name])); } catch (Exception e) { out.add('differ: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Prt__c>{ new Fern_Prt__c(Name = 'm5', Par__c = p1.Id, Flag__c = true, Stage__c = 'q', Score__c = 4), new Fern_Prt__c(Name = 'm6', Par__c = p2.Id, Flag__c = true, Stage__c = 'r', Score__c = 5), new Fern_Prt__c(Name = 'm7', Par__c = p1.Id, Flag__c = true, Stage__c = 's', Score__c = 6) }; out.add('twoparents: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pa__c  ORDER BY Name])); } catch (Exception e) { out.add('twoparents: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Prt__c>{ new Fern_Prt__c(Name = 'm8', Par__c = p1.Id, Flag__c = true, Stage__c = 'g', Score__c = 7), new Fern_Prt__c(Name = 'm9', Par__c = p2.Id, Flag__c = true, Stage__c = 'h', Score__c = 99) }; out.add('onefails: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pa__c  ORDER BY Name])); } catch (Exception e) { out.add('onefails: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Prt__c>{ new Fern_Prt__c(Name = 'n1', Par__c = p1.Id, Flag__c = true, Stage__c = 'i', Score__c = 8), new Fern_Prt__c(Name = 'n2', Par__c = p2.Id, Flag__c = true, Stage__c = 'j', Score__c = 99), new Fern_Prt__c(Name = 'n3', Par__c = p1.Id, Flag__c = true, Stage__c = 'k', Score__c = 9) }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pa__c  ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Prt__c>{ new Fern_Prt__c(Name = 'n4', Par__c = p1.Id, Flag__c = true, Stage__c = 'l', Score__c = 99), new Fern_Prt__c(Name = 'n5', Par__c = p1.Id, Flag__c = true, Stage__c = 'm', Score__c = 10), new Fern_Prt__c(Name = 'n6', Par__c = p2.Id, Flag__c = true, Stage__c = 'n', Score__c = 11) }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('sharedfail: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Pa__c  ORDER BY Name])); } catch (Exception e) { out.add('sharedfail: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String sibmerge() {
        List<String> out = new List<String>{ 'api=62.0' };
        insert new Fern_Sib__c(Name = 't0', Stage__c = 'target'); FernLog.take();
        try { insert new List<Fern_Sib__c>{ new Fern_Sib__c(Name = 't1', Stage__c = 'trig'), new Fern_Sib__c(Name = 't2', Stage__c = 'trig') }; out.add('two: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Sib__c  ORDER BY Name])); } catch (Exception e) { out.add('two: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String flow2() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Fl2__c(Name = 'g1'); out.add('one: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fl2__c WHERE Name='g1' ORDER BY Name])); } catch (Exception e) { out.add('one: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Fl2__c>{ new Fern_Fl2__c(Name = 'g2'), new Fern_Fl2__c(Name = 'g3'), new Fern_Fl2__c(Name = 'g4') }; out.add('bulk: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fl2__c WHERE Name IN ('g2','g3','g4') ORDER BY Name])); } catch (Exception e) { out.add('bulk: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String asn() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Asn__c(Name = 'a1', Stage__c = 'a', Score__c = 0); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asn__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Asn__c r = [SELECT Id FROM Fern_Asn__c LIMIT 1]; r.Flag__c = true; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asn__c  ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { delete [SELECT Id FROM Fern_Asn__c]; insert new List<Fern_Asn__c>{ new Fern_Asn__c(Name = 'a2', Stage__c = 'a', Score__c = 0), new Fern_Asn__c(Name = 'a3', Stage__c = 'b', Score__c = 0) }; out.add('bulk: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asn__c  ORDER BY Name])); } catch (Exception e) { out.add('bulk: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String asu() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Asu__c(Name = 'u1', Stage__c = 'a'); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asu__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Fern_Asu__c r = [SELECT Id FROM Fern_Asu__c LIMIT 1]; r.Note__c = 'n'; update r; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asu__c  ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String errups() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { List<Fern_Err__c> rows = new List<Fern_Err__c>{ new Fern_Err__c(Name = 'w1', Stage__c = 'ok'), new Fern_Err__c(Name = 'w2', Stage__c = 'bad'), new Fern_Err__c(Name = 'w3', Stage__c = 'ok') }; Database.UpsertResult[] rs = Database.upsert(rows, false); for (Database.UpsertResult s : rs) { out.add('ur=' + s.isSuccess() + '/' + s.isCreated() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode())); } out.add('new: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Err__c WHERE Name LIKE 'w%' ORDER BY Name])); } catch (Exception e) { out.add('new: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String fanmulti() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Fa2__c>{ new Fern_Fa2__c(Name = 'a1', Stage__c = 'ok'), new Fern_Fa2__c(Name = 'a2', Stage__c = 'bad'), new Fern_Fa2__c(Name = 'a3', Stage__c = 'ok') }; out.add('aon: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fa2__c  ORDER BY Name])); } catch (Exception e) { out.add('aon: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Fa2__c>{ new Fern_Fa2__c(Name = 'c1', Stage__c = 'ok'), new Fern_Fa2__c(Name = 'c2', Stage__c = 'bad'), new Fern_Fa2__c(Name = 'c3', Stage__c = 'ok') }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fa2__c  ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String asubulk() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Asu__c>{ new Fern_Asu__c(Name = 'v1', Stage__c = 'a'), new Fern_Asu__c(Name = 'v2', Stage__c = 'b') }; out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asu__c WHERE Name LIKE 'v%' ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { List<Fern_Asu__c> rows = [SELECT Id FROM Fern_Asu__c WHERE Name LIKE 'v%' ORDER BY Name]; rows[0].Note__c = 'x'; rows[1].Note__c = 'y'; update rows; out.add('upd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Asu__c WHERE Name LIKE 'v%' ORDER BY Name])); } catch (Exception e) { out.add('upd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String ordsaves() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Ord__c>{ new Fern_Ord__c(Name = 'q1', Stage__c = 'a'), new Fern_Ord__c(Name = 'q2', Stage__c = 'b') }; FernLog.take(); List<Fern_Ord__c> rows = [SELECT Id FROM Fern_Ord__c WHERE Name LIKE 'q%' ORDER BY Name]; rows[0].Stage__c = 'c'; rows[1].Stage__c = 'd'; update rows; out.add('bulkupd: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c WHERE Name LIKE 'q%' ORDER BY Name])); } catch (Exception e) { out.add('bulkupd: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Ord__c>{ new Fern_Ord__c(Name = 'q3', Stage__c = 'a'), new Fern_Ord__c(Name = 'q4', Stage__c = 'b') }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c WHERE Name IN ('q3','q4') ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { List<Fern_Ord__c> rows = new List<Fern_Ord__c>{ new Fern_Ord__c(Name = 'q5', Stage__c = 'a'), new Fern_Ord__c(Name = 'q6', Stage__c = 'b') }; upsert rows; out.add('upsert: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c WHERE Name IN ('q5','q6') ORDER BY Name])); } catch (Exception e) { out.add('upsert: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String flow2fail() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Fl2__c>{ new Fern_Fl2__c(Name = 'h1', Stage__c = 'ok'), new Fern_Fl2__c(Name = 'h2', Stage__c = 'bad') }; out.add('aon: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fl2__c WHERE Name LIKE 'h%' ORDER BY Name])); } catch (Exception e) { out.add('aon: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { Database.SaveResult[] rs = Database.insert(new List<Fern_Fl2__c>{ new Fern_Fl2__c(Name = 'h3', Stage__c = 'ok'), new Fern_Fl2__c(Name = 'h4', Stage__c = 'bad') }, false); for (Database.SaveResult s : rs) {{ out.add('sr=' + s.isSuccess() + (s.getErrors().isEmpty() ? '' : ':' + s.getErrors()[0].getStatusCode() + ':' + FernLog.failure(new DmlException(s.getErrors()[0].getMessage())))); }} out.add('partial: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fl2__c WHERE Name LIKE 'h%' ORDER BY Name])); } catch (Exception e) { out.add('partial: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String tpl() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Tpl__c(Name = 't1'); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Tpl__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String eml() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Eml__c(Name = 'e1'); out.add('emails=' + Limits.getEmailInvocations()); out.add('ins: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Eml__c  ORDER BY Name])); } catch (Exception e) { out.add('ins: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String multiflow() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Ord__c(Name = 'm1', Stage__c = 'a'); out.add('one: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c WHERE Name='m1' ORDER BY Name])); } catch (Exception e) { out.add('one: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new List<Fern_Ord__c>{ new Fern_Ord__c(Name = 'm2', Stage__c = 'a'), new Fern_Ord__c(Name = 'm3', Stage__c = 'b'), new Fern_Ord__c(Name = 'm4', Stage__c = 'c') }; out.add('bulk: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c WHERE Name IN ('m2','m3','m4') ORDER BY Name])); } catch (Exception e) { out.add('bulk: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { List<Fern_Ord__c> rows = [SELECT Id FROM Fern_Ord__c WHERE Name IN ('m2','m3','m4') ORDER BY Name]; for(Fern_Ord__c r:rows) r.Stage__c='d'; update rows; out.add('update: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Ord__c WHERE Name IN ('m2','m3','m4') ORDER BY Name])); } catch (Exception e) { out.add('update: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String globalcontrol() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Gbl__c(Name = 'g1', Stage__c = 'go'); out.add('yes: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Gbl__c WHERE Name='g1' ORDER BY Name])); } catch (Exception e) { out.add('yes: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Gbl__c(Name = 'g2', Stage__c = 'stop'); out.add('no: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Gbl__c WHERE Name='g2' ORDER BY Name])); } catch (Exception e) { out.add('no: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String branchedflow() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Fgd__c>{ new Fern_Fgd__c(Name = 'f1', Stage__c = 'go'), new Fern_Fgd__c(Name = 'f2', Stage__c = 'go') }; out.add('yes: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fgd__c  ORDER BY Name])); } catch (Exception e) { out.add('yes: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Fgd__c(Name = 'f3', Stage__c = 'stop'); out.add('no: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fgd__c WHERE Name='f3' ORDER BY Name])); } catch (Exception e) { out.add('no: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String routedcontrol() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Rgd__c>{ new Fern_Rgd__c(Name = 'r1', Stage__c = 'a'), new Fern_Rgd__c(Name = 'r2', Stage__c = 'b'), new Fern_Rgd__c(Name = 'r3', Stage__c = 'c'), new Fern_Rgd__c(Name = 'r4', Stage__c = 'd'), new Fern_Rgd__c(Name = 'r5', Stage__c = 'miss') }; out.add('mix: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rgd__c  ORDER BY Name])); } catch (Exception e) { out.add('mix: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String routedsame() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Rgs__c>{ new Fern_Rgs__c(Name = 'r1', Stage__c = 'a'), new Fern_Rgs__c(Name = 'r2', Stage__c = 'b'), new Fern_Rgs__c(Name = 'r3', Stage__c = 'c'), new Fern_Rgs__c(Name = 'r4', Stage__c = 'd'), new Fern_Rgs__c(Name = 'r5', Stage__c = 'miss') }; out.add('mix: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Rgs__c  ORDER BY Name])); } catch (Exception e) { out.add('mix: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String userclock() {
        List<String> out = new List<String>{ 'api=62.0' };
        Fern_Gbi__c a=new Fern_Gbi__c(Name='u',Stage__c='uid'); insert a;
        Fern_Gbi__c b=new Fern_Gbi__c(Name='d',Stage__c='date'); insert b;
        Datetime beforeTime=System.now(); Fern_Gbi__c c=new Fern_Gbi__c(Name='n',Stage__c='stamp'); insert c; Datetime afterTime=System.now();
        Fern_Gbi__c d=new Fern_Gbi__c(Name='x',Stage__c='stop'); insert d;
        Set<Id> ids = new Set<Id>{a.Id,b.Id,c.Id,d.Id}; Map<Id,Fern_Gbi__c> got=new Map<Id,Fern_Gbi__c>([SELECT Id,Note__c,Day__c,Stamp__c FROM Fern_Gbi__c WHERE Id IN :ids]);
        String uid=String.valueOf(UserInfo.getUserId()); String value=got.get(a.Id).Note__c;
        out.add('user='+(value==uid ? 'full' : value==uid.substring(0,15) ? 'short' : value==null ? 'null' : 'other'));
        out.add('today='+(got.get(b.Id).Day__c==Date.today()));
        out.add('now='+(got.get(c.Id).Stamp__c!=null && got.get(c.Id).Stamp__c>=beforeTime.addSeconds(-5) && got.get(c.Id).Stamp__c<=afterTime.addSeconds(5)));
        out.add('control='+(got.get(d.Id).Note__c==null && got.get(d.Id).Day__c==null && got.get(d.Id).Stamp__c==null));
        return String.join(out, ' | ');
    }
    public static String booleanconstant() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new Fern_Gbf__c(Name = 'b1', Stage__c = 'go'); out.add('yes: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Gbf__c WHERE Name='b1' ORDER BY Name])); } catch (Exception e) { out.add('yes: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        try { insert new Fern_Gbf__c(Name = 'b2', Stage__c = 'stop'); out.add('no: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Gbf__c WHERE Name='b2' ORDER BY Name])); } catch (Exception e) { out.add('no: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String pickchanged() {
        List<String> out = new List<String>{ 'api=62.0' };
        Fern_Gpk__c a=new Fern_Gpk__c(Name='a',Choice__c='A',Flag__c=false); Fern_Gpk__c b=new Fern_Gpk__c(Name='b',Choice__c='A',Flag__c=true); insert new List<Fern_Gpk__c>{a,b}; FernLog.take();
        a.Choice__c='B'; b.Choice__c='B'; update new List<Fern_Gpk__c>{a,b};
        Map<Id,Fern_Gpk__c> got=new Map<Id,Fern_Gpk__c>([SELECT Id,Note__c FROM Fern_Gpk__c WHERE Name IN ('a','b')]);
        out.add('matched='+(got.get(a.Id).Note__c=='PICK')+';blocked='+(got.get(b.Id).Note__c==null));
        a.Note__c=null; update a; Fern_Gpk__c again=[SELECT Note__c FROM Fern_Gpk__c WHERE Id=:a.Id]; out.add('unchanged='+(again.Note__c==null));
        Fern_Gpk__c c=new Fern_Gpk__c(Name='c',Choice__c='B',Flag__c=false);insert c;c.Choice__c='A';update c;Fern_Gpk__c other=[SELECT Note__c FROM Fern_Gpk__c WHERE Id=:c.Id];out.add('other='+(other.Note__c==null));
        return String.join(out, ' | ');
    }
    public static String userlookup() {
        List<String> out = new List<String>{ 'api=62.0' };
        Case a=new Case(Subject='go',Origin='Web',Status='New');Case b=new Case(Subject='stop',Origin='Web',Status='New');insert new List<Case>{a,b};
        Set<Id> ids=new Set<Id>{a.Id,b.Id}; Map<Id,Case> got=new Map<Id,Case>([SELECT Id,Fern_Actor__c FROM Case WHERE Id IN :ids]);out.add('match='+(got.get(a.Id).Fern_Actor__c==UserInfo.getUserId())+';control='+(got.get(b.Id).Fern_Actor__c==null));
        return String.join(out, ' | ');
    }
    public static String twoactions() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Fma__c>{ new Fern_Fma__c(Name = 'a', Stage__c = 'go'), new Fern_Fma__c(Name = 'b', Stage__c = 'go'), new Fern_Fma__c(Name = 'c', Stage__c = 'stop') }; out.add('mixed: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fma__c  ORDER BY Name])); } catch (Exception e) { out.add('mixed: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String twolookups() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Fml__c>{ new Fern_Fml__c(Name = 'a', Stage__c = 'go'), new Fern_Fml__c(Name = 'b', Stage__c = 'go'), new Fern_Fml__c(Name = 'c', Stage__c = 'stop') }; out.add('mixed: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Fml__c  ORDER BY Name])); } catch (Exception e) { out.add('mixed: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String fourflows() {
        List<String> out = new List<String>{ 'api=62.0' };
        try { insert new List<Fern_Faf__c>{ new Fern_Faf__c(Name = 'a', Stage__c = 'go'), new Fern_Faf__c(Name = 'b', Stage__c = 'go'), new Fern_Faf__c(Name = 'c', Stage__c = 'stop') }; out.add('mixed: ' + FernLog.take() + ' => ' + FernCasesHELP.rows([SELECT Stage__c, Note__c, Old__c, Score__c, Flag__c, Day__c FROM Fern_Faf__c  ORDER BY Name])); } catch (Exception e) { out.add('mixed: ' + FernLog.take() + ' !! ' + FernLog.failure(e)); }
        return String.join(out, ' | ');
    }
    public static String lookupflow() { List<String> out=new List<String>{'api=62.0'}; List<Fern_Root__c> roots=new List<Fern_Root__c>{new Fern_Root__c(Name='one'),new Fern_Root__c(Name='two')}; insert roots; FernLog.take(); List<Contact> made=new List<Contact>{new Contact(FirstName='a',LastName='go',Fern_Root_Link__c=roots[0].Id),new Contact(FirstName='b',LastName='go',Fern_Root_Link__c=roots[1].Id),new Contact(FirstName='c',LastName='stop',Fern_Root_Link__c=roots[1].Id)}; insert made; Set<Id> ids=new Set<Id>{made[0].Id,made[1].Id,made[2].Id}; List<Contact> a=[SELECT FirstName,LastName,Description FROM Contact WHERE Id IN :ids ORDER BY FirstName]; out.add('mixed: '+FernLog.take()+' => '+a[0].FirstName+':'+a[0].Description+';'+a[1].FirstName+':'+a[1].Description+';'+a[2].FirstName+':'+a[2].Description); return String.join(out,' | '); }
}
