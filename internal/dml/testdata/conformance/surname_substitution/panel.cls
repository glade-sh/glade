public class SurnameBehavior {
    private static String hits(Datacloud.DuplicateResult rule, Map<Id,String> labels) {
        List<String> found = new List<String>();
        for (Datacloud.MatchResult result : rule.getMatchResults()) {
            for (Datacloud.MatchRecord match : result.getMatchRecords()) {
                Id matchedId = match.getRecord().Id;
                System.assert(labels.containsKey(matchedId), 'unidentified seed');
                Double raw = match.getMatchConfidence();
                found.add(labels.get(matchedId) + '=' + String.valueOf(raw.intValue()) + '~' + String.valueOf(raw));
            }
        }
        found.sort();
        return found.isEmpty() ? '-' : String.join(found, '+');
    }
    private static SObject build(String obj, String first, String last, String email, String phone, String key) {
        SObject s = obj == 'Lead' ? (SObject) new Lead() : (SObject) new Contact();
        s.put('FirstName', first);
        s.put('LastName', last);
        if (email != null) s.put('Email', email);
        if (phone != null) s.put('Phone', phone);
        if (obj == 'Lead') s.put('Company', 'Cmp' + key);
        return s;
    }
    public static String run(String obj, Integer startKey, List<String> specs) {
        String ruleName = 'Standard_' + obj + '_Duplicate_Rule';
        List<String> all = new List<String>(specs);
        all.add('Orzabal/Orzabal/std');
        all.add('Zimbalist/Okonkwo/std');
        List<SObject> seeds = (List<SObject>) Type.forName('List<' + obj + '>').newInstance();
        List<SObject> candidates = (List<SObject>) Type.forName('List<' + obj + '>').newInstance();
        for (Integer i = 0; i < all.size(); i++) {
            List<String> p = all[i].split('/');
            String v = p[2];
            String key = String.valueOf(startKey + i);
            String email = 's' + key + '@k' + key + '.example';
            String phone = '(630) 555-' + String.valueOf(1000 + Math.mod(startKey + i, 9000));
            String sf = 'Brannock', cf = 'Brannock';
            String se = email, ce = email, sp = null, cp = null;
            if (v == 'noemail') { se = null; ce = null; }
            else if (v == 'diffemail') { ce = 'c' + key + '@z' + key + '.example'; }
            else if (v == 'diffname') { cf = 'Ormerod'; }
            else if (v == 'phone') { se = null; ce = null; sp = phone; cp = phone; }
            else System.assertEquals('std', v);
            seeds.add(build(obj, sf, p[0], se, sp, key));
            candidates.add(build(obj, cf, p[1], ce, cp, key));
        }
        insert seeds;
        Map<Id,String> labels = new Map<Id,String>();
        for (Integer i = 0; i < seeds.size(); i++) labels.put(seeds[i].Id, String.valueOf(i));
        List<Datacloud.FindDuplicatesResult> found = Datacloud.FindDuplicates.findDuplicates(candidates);
        System.assertEquals(all.size(), found.size(), 'incomplete Find response');
        Database.SaveResult[] saved = Database.insert(candidates, false);
        List<String> outcomes = new List<String>();
        Integer savedCount = 0;
        for (Integer i = 0; i < all.size(); i++) {
            List<Datacloud.DuplicateResult> rules = found[i].getDuplicateResults();
            System.assertEquals(1, rules.size(), 'one default rule expected');
            System.assertEquals(ruleName, rules[0].getDuplicateRule());
            String f = hits(rules[0], labels);
            String d = 'S';
            if (saved[i].isSuccess()) savedCount++;
            else {
                Database.Error[] errs = saved[i].getErrors();
                System.assertEquals(1, errs.size());
                System.assertEquals('DUPLICATES_DETECTED', String.valueOf(errs[0].getStatusCode()));
                Datacloud.DuplicateResult dr = ((Database.DuplicateError) errs[0]).getDuplicateResult();
                System.assertEquals(ruleName, dr.getDuplicateRule());
                d = hits(dr, labels) + (dr.isAllowSave() ? '' : '!');
            }
            outcomes.add(String.valueOf(i) + ':' + f + '/' + d);
        }
        System.assert(outcomes[outcomes.size() - 2].contains('='), 'positive control absent');
        System.assert(outcomes[outcomes.size() - 1].endsWith(':-/S'), 'negative control matched');
        return ruleName + '|saved=' + String.valueOf(savedCount) + '|' + String.join(outcomes, ',');
    }
    public static String m_J_Contact_first_row_fwd() {
        return run('Contact', 1000, new List<String>{
            'Mtpu/Ntpu/std',
            'Siyd/Diyd/std',
            'Mivjl/Nivjl/std',
            'Svbqp/Dvbqp/std',
            'Mhojhy/Nhojhy/std',
            'Sgfdmi/Dgfdmi/std',
            'Mfaqeyr/Nfaqeyr/std',
            'Svvndab/Dvvndab/std',
            'Mtepohuh/Ntepohuh/std',
            'Smqvcyld/Dmqvcyld/std'
        });
    }
    public static String m_J_Contact_first_row_rev() {
        return run('Contact', 1050, new List<String>{
            'Ntpu/Mtpu/std',
            'Diyd/Siyd/std',
            'Nivjl/Mivjl/std',
            'Dvbqp/Svbqp/std',
            'Nhojhy/Mhojhy/std',
            'Dgfdmi/Sgfdmi/std',
            'Nfaqeyr/Mfaqeyr/std',
            'Dvvndab/Svvndab/std',
            'Ntepohuh/Mtepohuh/std',
            'Dmqvcyld/Smqvcyld/std'
        });
    }
    public static String m_J_Lead_first_row_fwd() {
        return run('Lead', 1100, new List<String>{
            'Mtpu/Ntpu/std',
            'Siyd/Diyd/std',
            'Mivjl/Nivjl/std',
            'Svbqp/Dvbqp/std',
            'Mhojhy/Nhojhy/std',
            'Sgfdmi/Dgfdmi/std',
            'Mfaqeyr/Nfaqeyr/std',
            'Svvndab/Dvvndab/std',
            'Mtepohuh/Ntepohuh/std',
            'Smqvcyld/Dmqvcyld/std'
        });
    }
    public static String m_J_Lead_first_row_rev() {
        return run('Lead', 1150, new List<String>{
            'Ntpu/Mtpu/std',
            'Diyd/Siyd/std',
            'Nivjl/Mivjl/std',
            'Dvbqp/Svbqp/std',
            'Nhojhy/Mhojhy/std',
            'Dgfdmi/Sgfdmi/std',
            'Nfaqeyr/Mfaqeyr/std',
            'Dvvndab/Svvndab/std',
            'Ntepohuh/Mtepohuh/std',
            'Dmqvcyld/Smqvcyld/std'
        });
    }
    public static String m_J_Contact_first_vertical_fwd() {
        return run('Contact', 1200, new List<String>{
            'Qxbf/Axbf/std',
            'Tuzv/Guzv/std',
            'Qvmlg/Avmlg/std',
            'Ttnky/Gtnky/std',
            'Qixbij/Aixbij/std',
            'Tjiiha/Gjiiha/std',
            'Qrsgqhz/Arsgqhz/std',
            'Tyjjwgf/Gyjjwgf/std',
            'Qarozxei/Aarozxei/std',
            'Thkqqpzb/Ghkqqpzb/std'
        });
    }
    public static String m_J_Contact_first_vertical_rev() {
        return run('Contact', 1250, new List<String>{
            'Axbf/Qxbf/std',
            'Guzv/Tuzv/std',
            'Avmlg/Qvmlg/std',
            'Gtnky/Ttnky/std',
            'Aixbij/Qixbij/std',
            'Gjiiha/Tjiiha/std',
            'Arsgqhz/Qrsgqhz/std',
            'Gyjjwgf/Tyjjwgf/std',
            'Aarozxei/Qarozxei/std',
            'Ghkqqpzb/Thkqqpzb/std'
        });
    }
    public static String m_J_Lead_first_vertical_fwd() {
        return run('Lead', 1300, new List<String>{
            'Qxbf/Axbf/std',
            'Tuzv/Guzv/std',
            'Qvmlg/Avmlg/std',
            'Ttnky/Gtnky/std',
            'Qixbij/Aixbij/std',
            'Tjiiha/Gjiiha/std',
            'Qrsgqhz/Arsgqhz/std',
            'Tyjjwgf/Gyjjwgf/std',
            'Qarozxei/Aarozxei/std',
            'Thkqqpzb/Ghkqqpzb/std'
        });
    }
    public static String m_J_Lead_first_vertical_rev() {
        return run('Lead', 1350, new List<String>{
            'Axbf/Qxbf/std',
            'Guzv/Tuzv/std',
            'Avmlg/Qvmlg/std',
            'Gtnky/Ttnky/std',
            'Aixbij/Qixbij/std',
            'Gjiiha/Tjiiha/std',
            'Arsgqhz/Qrsgqhz/std',
            'Gyjjwgf/Tyjjwgf/std',
            'Aarozxei/Qarozxei/std',
            'Ghkqqpzb/Thkqqpzb/std'
        });
    }
    public static String m_J_Contact_first_diagonal_fwd() {
        return run('Contact', 1400, new List<String>{
            'Wuhv/Duhv/std',
            'Fhqu/Chqu/std',
            'Wycmy/Dycmy/std',
            'Fuorv/Cuorv/std',
            'Wstdbr/Dstdbr/std',
            'Fbmiew/Cbmiew/std',
            'Wmucfak/Dmucfak/std',
            'Foeqpiu/Coeqpiu/std',
            'Wrcwkevy/Drcwkevy/std',
            'Fnztsfth/Cnztsfth/std'
        });
    }
    public static String m_J_Contact_first_diagonal_rev() {
        return run('Contact', 1450, new List<String>{
            'Duhv/Wuhv/std',
            'Chqu/Fhqu/std',
            'Dycmy/Wycmy/std',
            'Cuorv/Fuorv/std',
            'Dstdbr/Wstdbr/std',
            'Cbmiew/Fbmiew/std',
            'Dmucfak/Wmucfak/std',
            'Coeqpiu/Foeqpiu/std',
            'Drcwkevy/Wrcwkevy/std',
            'Cnztsfth/Fnztsfth/std'
        });
    }
    public static String m_J_Lead_first_diagonal_fwd() {
        return run('Lead', 1500, new List<String>{
            'Wuhv/Duhv/std',
            'Fhqu/Chqu/std',
            'Wycmy/Dycmy/std',
            'Fuorv/Cuorv/std',
            'Wstdbr/Dstdbr/std',
            'Fbmiew/Cbmiew/std',
            'Wmucfak/Dmucfak/std',
            'Foeqpiu/Coeqpiu/std',
            'Wrcwkevy/Drcwkevy/std',
            'Fnztsfth/Cnztsfth/std'
        });
    }
    public static String m_J_Lead_first_diagonal_rev() {
        return run('Lead', 1550, new List<String>{
            'Duhv/Wuhv/std',
            'Chqu/Fhqu/std',
            'Dycmy/Wycmy/std',
            'Cuorv/Fuorv/std',
            'Dstdbr/Wstdbr/std',
            'Cbmiew/Fbmiew/std',
            'Dmucfak/Wmucfak/std',
            'Coeqpiu/Foeqpiu/std',
            'Drcwkevy/Wrcwkevy/std',
            'Cnztsfth/Fnztsfth/std'
        });
    }
    public static String m_J_Contact_first_far_fwd() {
        return run('Contact', 1600, new List<String>{
            'Zrwa/Mrwa/std',
            'Qzhi/Pzhi/std',
            'Zroib/Mroib/std',
            'Qadmk/Padmk/std',
            'Zzideq/Mzideq/std',
            'Qkovcm/Pkovcm/std',
            'Zlafyuv/Mlafyuv/std',
            'Qzfzogt/Pzfzogt/std',
            'Zzhmgsgi/Mzhmgsgi/std',
            'Qygjjniu/Pygjjniu/std'
        });
    }
    public static String m_J_Contact_first_far_rev() {
        return run('Contact', 1650, new List<String>{
            'Mrwa/Zrwa/std',
            'Pzhi/Qzhi/std',
            'Mroib/Zroib/std',
            'Padmk/Qadmk/std',
            'Mzideq/Zzideq/std',
            'Pkovcm/Qkovcm/std',
            'Mlafyuv/Zlafyuv/std',
            'Pzfzogt/Qzfzogt/std',
            'Mzhmgsgi/Zzhmgsgi/std',
            'Pygjjniu/Qygjjniu/std'
        });
    }
    public static String m_J_Lead_first_far_fwd() {
        return run('Lead', 1700, new List<String>{
            'Zrwa/Mrwa/std',
            'Qzhi/Pzhi/std',
            'Zroib/Mroib/std',
            'Qadmk/Padmk/std',
            'Zzideq/Mzideq/std',
            'Qkovcm/Pkovcm/std',
            'Zlafyuv/Mlafyuv/std',
            'Qzfzogt/Pzfzogt/std',
            'Zzhmgsgi/Mzhmgsgi/std',
            'Qygjjniu/Pygjjniu/std'
        });
    }
    public static String m_J_Lead_first_far_rev() {
        return run('Lead', 1750, new List<String>{
            'Mrwa/Zrwa/std',
            'Pzhi/Qzhi/std',
            'Mroib/Zroib/std',
            'Padmk/Qadmk/std',
            'Mzideq/Zzideq/std',
            'Pkovcm/Qkovcm/std',
            'Mlafyuv/Zlafyuv/std',
            'Pzfzogt/Qzfzogt/std',
            'Mzhmgsgi/Zzhmgsgi/std',
            'Pygjjniu/Qygjjniu/std'
        });
    }
    public static String m_J_Contact_middle_row_fwd() {
        return run('Contact', 1800, new List<String>{
            'Yomk/Yonk/std',
            'Tbst/Tbdt/std',
            'Tpmxi/Tpnxi/std',
            'Hoshr/Hodhr/std',
            'Diemia/Dienia/std',
            'Ljdstn/Ljddtn/std',
            'Slwmpmk/Slwnpmk/std',
            'Xblsznv/Xbldznv/std',
            'Zzadmfmf/Zzadnfmf/std',
            'Vebgsvxv/Vebgdvxv/std'
        });
    }
    public static String m_J_Contact_middle_row_rev() {
        return run('Contact', 1850, new List<String>{
            'Yonk/Yomk/std',
            'Tbdt/Tbst/std',
            'Tpnxi/Tpmxi/std',
            'Hodhr/Hoshr/std',
            'Dienia/Diemia/std',
            'Ljddtn/Ljdstn/std',
            'Slwnpmk/Slwmpmk/std',
            'Xbldznv/Xblsznv/std',
            'Zzadnfmf/Zzadmfmf/std',
            'Vebgdvxv/Vebgsvxv/std'
        });
    }
    public static String m_J_Lead_middle_row_fwd() {
        return run('Lead', 1900, new List<String>{
            'Yomk/Yonk/std',
            'Tbst/Tbdt/std',
            'Tpmxi/Tpnxi/std',
            'Hoshr/Hodhr/std',
            'Diemia/Dienia/std',
            'Ljdstn/Ljddtn/std',
            'Slwmpmk/Slwnpmk/std',
            'Xblsznv/Xbldznv/std',
            'Zzadmfmf/Zzadnfmf/std',
            'Vebgsvxv/Vebgdvxv/std'
        });
    }
    public static String m_J_Lead_middle_row_rev() {
        return run('Lead', 1950, new List<String>{
            'Yonk/Yomk/std',
            'Tbdt/Tbst/std',
            'Tpnxi/Tpmxi/std',
            'Hodhr/Hoshr/std',
            'Dienia/Diemia/std',
            'Ljddtn/Ljdstn/std',
            'Slwnpmk/Slwmpmk/std',
            'Xbldznv/Xblsznv/std',
            'Zzadnfmf/Zzadmfmf/std',
            'Vebgdvxv/Vebgsvxv/std'
        });
    }
    public static String m_J_Contact_middle_vertical_fwd() {
        return run('Contact', 2000, new List<String>{
            'Ugqk/Ugak/std',
            'Pstu/Psgu/std',
            'Daqez/Daaez/std',
            'Gxthd/Gxghd/std',
            'Gxgqoh/Gxgaoh/std',
            'Mfctbr/Mfcgbr/std',
            'Mokqqju/Mokaqju/std',
            'Srctuos/Srcguos/std',
            'Heqsqqee/Heqsaqee/std',
            'Mpdutxrn/Mpdugxrn/std'
        });
    }
    public static String m_J_Contact_middle_vertical_rev() {
        return run('Contact', 2050, new List<String>{
            'Ugak/Ugqk/std',
            'Psgu/Pstu/std',
            'Daaez/Daqez/std',
            'Gxghd/Gxthd/std',
            'Gxgaoh/Gxgqoh/std',
            'Mfcgbr/Mfctbr/std',
            'Mokaqju/Mokqqju/std',
            'Srcguos/Srctuos/std',
            'Heqsaqee/Heqsqqee/std',
            'Mpdugxrn/Mpdutxrn/std'
        });
    }
    public static String m_J_Lead_middle_vertical_fwd() {
        return run('Lead', 2100, new List<String>{
            'Ugqk/Ugak/std',
            'Pstu/Psgu/std',
            'Daqez/Daaez/std',
            'Gxthd/Gxghd/std',
            'Gxgqoh/Gxgaoh/std',
            'Mfctbr/Mfcgbr/std',
            'Mokqqju/Mokaqju/std',
            'Srctuos/Srcguos/std',
            'Heqsqqee/Heqsaqee/std',
            'Mpdutxrn/Mpdugxrn/std'
        });
    }
    public static String m_J_Lead_middle_vertical_rev() {
        return run('Lead', 2150, new List<String>{
            'Ugak/Ugqk/std',
            'Psgu/Pstu/std',
            'Daaez/Daqez/std',
            'Gxghd/Gxthd/std',
            'Gxgaoh/Gxgqoh/std',
            'Mfcgbr/Mfctbr/std',
            'Mokaqju/Mokqqju/std',
            'Srcguos/Srctuos/std',
            'Heqsaqee/Heqsqqee/std',
            'Mpdugxrn/Mpdutxrn/std'
        });
    }
    public static String m_J_Contact_middle_diagonal_fwd() {
        return run('Contact', 2200, new List<String>{
            'Dqwr/Dqdr/std',
            'Rffl/Rfcl/std',
            'Ggwbl/Ggdbl/std',
            'Dbfxz/Dbcxz/std',
            'Yquwem/Yqudem/std',
            'Qwefxx/Qwecxx/std',
            'Mnnwfsy/Mnndfsy/std',
            'Qtgfvmc/Qtgcvmc/std',
            'Ghxlwqca/Ghxldqca/std',
            'Ewoyfweg/Ewoycweg/std'
        });
    }
    public static String m_J_Contact_middle_diagonal_rev() {
        return run('Contact', 2250, new List<String>{
            'Dqdr/Dqwr/std',
            'Rfcl/Rffl/std',
            'Ggdbl/Ggwbl/std',
            'Dbcxz/Dbfxz/std',
            'Yqudem/Yquwem/std',
            'Qwecxx/Qwefxx/std',
            'Mnndfsy/Mnnwfsy/std',
            'Qtgcvmc/Qtgfvmc/std',
            'Ghxldqca/Ghxlwqca/std',
            'Ewoycweg/Ewoyfweg/std'
        });
    }
    public static String m_J_Lead_middle_diagonal_fwd() {
        return run('Lead', 2300, new List<String>{
            'Dqwr/Dqdr/std',
            'Rffl/Rfcl/std',
            'Ggwbl/Ggdbl/std',
            'Dbfxz/Dbcxz/std',
            'Yquwem/Yqudem/std',
            'Qwefxx/Qwecxx/std',
            'Mnnwfsy/Mnndfsy/std',
            'Qtgfvmc/Qtgcvmc/std',
            'Ghxlwqca/Ghxldqca/std',
            'Ewoyfweg/Ewoycweg/std'
        });
    }
    public static String m_J_Lead_middle_diagonal_rev() {
        return run('Lead', 2350, new List<String>{
            'Dqdr/Dqwr/std',
            'Rfcl/Rffl/std',
            'Ggdbl/Ggwbl/std',
            'Dbcxz/Dbfxz/std',
            'Yqudem/Yquwem/std',
            'Qwecxx/Qwefxx/std',
            'Mnndfsy/Mnnwfsy/std',
            'Qtgcvmc/Qtgfvmc/std',
            'Ghxldqca/Ghxlwqca/std',
            'Ewoycweg/Ewoyfweg/std'
        });
    }
    public static String m_J_Contact_middle_far_fwd() {
        return run('Contact', 2400, new List<String>{
            'Szzw/Szmw/std',
            'Drqg/Drpg/std',
            'Mlzxk/Mlmxk/std',
            'Tvqmy/Tvpmy/std',
            'Gwuzdv/Gwumdv/std',
            'Kokqdb/Kokpdb/std',
            'Kiqztbj/Kiqmtbj/std',
            'Vibqnwy/Vibpnwy/std',
            'Bmiezumq/Bmiemumq/std',
            'Bsbeqsae/Bsbepsae/std'
        });
    }
    public static String m_J_Contact_middle_far_rev() {
        return run('Contact', 2450, new List<String>{
            'Szmw/Szzw/std',
            'Drpg/Drqg/std',
            'Mlmxk/Mlzxk/std',
            'Tvpmy/Tvqmy/std',
            'Gwumdv/Gwuzdv/std',
            'Kokpdb/Kokqdb/std',
            'Kiqmtbj/Kiqztbj/std',
            'Vibpnwy/Vibqnwy/std',
            'Bmiemumq/Bmiezumq/std',
            'Bsbepsae/Bsbeqsae/std'
        });
    }
    public static String m_J_Lead_middle_far_fwd() {
        return run('Lead', 2500, new List<String>{
            'Szzw/Szmw/std',
            'Drqg/Drpg/std',
            'Mlzxk/Mlmxk/std',
            'Tvqmy/Tvpmy/std',
            'Gwuzdv/Gwumdv/std',
            'Kokqdb/Kokpdb/std',
            'Kiqztbj/Kiqmtbj/std',
            'Vibqnwy/Vibpnwy/std',
            'Bmiezumq/Bmiemumq/std',
            'Bsbeqsae/Bsbepsae/std'
        });
    }
    public static String m_J_Lead_middle_far_rev() {
        return run('Lead', 2550, new List<String>{
            'Szmw/Szzw/std',
            'Drpg/Drqg/std',
            'Mlmxk/Mlzxk/std',
            'Tvpmy/Tvqmy/std',
            'Gwumdv/Gwuzdv/std',
            'Kokpdb/Kokqdb/std',
            'Kiqmtbj/Kiqztbj/std',
            'Vibpnwy/Vibqnwy/std',
            'Bmiemumq/Bmiezumq/std',
            'Bsbepsae/Bsbeqsae/std'
        });
    }
    public static String m_J_Contact_final_row_fwd() {
        return run('Contact', 2600, new List<String>{
            'Nhgm/Nhgn/std',
            'Knvs/Knvd/std',
            'Rzodm/Rzodn/std',
            'Abnjs/Abnjd/std',
            'Yozjtm/Yozjtn/std',
            'Ytglxs/Ytglxd/std',
            'Fbnbflm/Fbnbfln/std',
            'Vfjckos/Vfjckod/std',
            'Jtuqmhsm/Jtuqmhsn/std',
            'Jnpqakvs/Jnpqakvd/std'
        });
    }
    public static String m_J_Contact_final_row_rev() {
        return run('Contact', 2650, new List<String>{
            'Nhgn/Nhgm/std',
            'Knvd/Knvs/std',
            'Rzodn/Rzodm/std',
            'Abnjd/Abnjs/std',
            'Yozjtn/Yozjtm/std',
            'Ytglxd/Ytglxs/std',
            'Fbnbfln/Fbnbflm/std',
            'Vfjckod/Vfjckos/std',
            'Jtuqmhsn/Jtuqmhsm/std',
            'Jnpqakvd/Jnpqakvs/std'
        });
    }
    public static String m_J_Lead_final_row_fwd() {
        return run('Lead', 2700, new List<String>{
            'Nhgm/Nhgn/std',
            'Knvs/Knvd/std',
            'Rzodm/Rzodn/std',
            'Abnjs/Abnjd/std',
            'Yozjtm/Yozjtn/std',
            'Ytglxs/Ytglxd/std',
            'Fbnbflm/Fbnbfln/std',
            'Vfjckos/Vfjckod/std',
            'Jtuqmhsm/Jtuqmhsn/std',
            'Jnpqakvs/Jnpqakvd/std'
        });
    }
    public static String m_J_Lead_final_row_rev() {
        return run('Lead', 2750, new List<String>{
            'Nhgn/Nhgm/std',
            'Knvd/Knvs/std',
            'Rzodn/Rzodm/std',
            'Abnjd/Abnjs/std',
            'Yozjtn/Yozjtm/std',
            'Ytglxd/Ytglxs/std',
            'Fbnbfln/Fbnbflm/std',
            'Vfjckod/Vfjckos/std',
            'Jtuqmhsn/Jtuqmhsm/std',
            'Jnpqakvd/Jnpqakvs/std'
        });
    }
    public static String m_J_Contact_final_vertical_fwd() {
        return run('Contact', 2800, new List<String>{
            'Zquq/Zqua/std',
            'Gqht/Gqhg/std',
            'Mfoqq/Mfoqa/std',
            'Wnfpt/Wnfpg/std',
            'Ziwctq/Ziwcta/std',
            'Sjdbst/Sjdbsg/std',
            'Wfdstsq/Wfdstsa/std',
            'Mjkuoot/Mjkuoog/std',
            'Ibbfputq/Ibbfputa/std',
            'Vxdkcpst/Vxdkcpsg/std'
        });
    }
    public static String m_J_Contact_final_vertical_rev() {
        return run('Contact', 2850, new List<String>{
            'Zqua/Zquq/std',
            'Gqhg/Gqht/std',
            'Mfoqa/Mfoqq/std',
            'Wnfpg/Wnfpt/std',
            'Ziwcta/Ziwctq/std',
            'Sjdbsg/Sjdbst/std',
            'Wfdstsa/Wfdstsq/std',
            'Mjkuoog/Mjkuoot/std',
            'Ibbfputa/Ibbfputq/std',
            'Vxdkcpsg/Vxdkcpst/std'
        });
    }
    public static String m_J_Lead_final_vertical_fwd() {
        return run('Lead', 2900, new List<String>{
            'Zquq/Zqua/std',
            'Gqht/Gqhg/std',
            'Mfoqq/Mfoqa/std',
            'Wnfpt/Wnfpg/std',
            'Ziwctq/Ziwcta/std',
            'Sjdbst/Sjdbsg/std',
            'Wfdstsq/Wfdstsa/std',
            'Mjkuoot/Mjkuoog/std',
            'Ibbfputq/Ibbfputa/std',
            'Vxdkcpst/Vxdkcpsg/std'
        });
    }
    public static String m_J_Lead_final_vertical_rev() {
        return run('Lead', 2950, new List<String>{
            'Zqua/Zquq/std',
            'Gqhg/Gqht/std',
            'Mfoqa/Mfoqq/std',
            'Wnfpg/Wnfpt/std',
            'Ziwcta/Ziwctq/std',
            'Sjdbsg/Sjdbst/std',
            'Wfdstsa/Wfdstsq/std',
            'Mjkuoog/Mjkuoot/std',
            'Ibbfputa/Ibbfputq/std',
            'Vxdkcpsg/Vxdkcpst/std'
        });
    }
    public static String m_J_Contact_final_diagonal_fwd() {
        return run('Contact', 3000, new List<String>{
            'Lifw/Lifd/std',
            'Waaf/Waac/std',
            'Hewvw/Hewvd/std',
            'Yxftf/Yxftc/std',
            'Ljtfww/Ljtfwd/std',
            'Vnafuf/Vnafuc/std',
            'Dfjdbkw/Dfjdbkd/std',
            'Hholorf/Hholorc/std',
            'Hyfwvrfw/Hyfwvrfd/std',
            'Umvaqchf/Umvaqchc/std'
        });
    }
    public static String m_J_Contact_final_diagonal_rev() {
        return run('Contact', 3050, new List<String>{
            'Lifd/Lifw/std',
            'Waac/Waaf/std',
            'Hewvd/Hewvw/std',
            'Yxftc/Yxftf/std',
            'Ljtfwd/Ljtfww/std',
            'Vnafuc/Vnafuf/std',
            'Dfjdbkd/Dfjdbkw/std',
            'Hholorc/Hholorf/std',
            'Hyfwvrfd/Hyfwvrfw/std',
            'Umvaqchc/Umvaqchf/std'
        });
    }
    public static String m_J_Lead_final_diagonal_fwd() {
        return run('Lead', 3100, new List<String>{
            'Lifw/Lifd/std',
            'Waaf/Waac/std',
            'Hewvw/Hewvd/std',
            'Yxftf/Yxftc/std',
            'Ljtfww/Ljtfwd/std',
            'Vnafuf/Vnafuc/std',
            'Dfjdbkw/Dfjdbkd/std',
            'Hholorf/Hholorc/std',
            'Hyfwvrfw/Hyfwvrfd/std',
            'Umvaqchf/Umvaqchc/std'
        });
    }
    public static String m_J_Lead_final_diagonal_rev() {
        return run('Lead', 3150, new List<String>{
            'Lifd/Lifw/std',
            'Waac/Waaf/std',
            'Hewvd/Hewvw/std',
            'Yxftc/Yxftf/std',
            'Ljtfwd/Ljtfww/std',
            'Vnafuc/Vnafuf/std',
            'Dfjdbkd/Dfjdbkw/std',
            'Hholorc/Hholorf/std',
            'Hyfwvrfd/Hyfwvrfw/std',
            'Umvaqchc/Umvaqchf/std'
        });
    }
    public static String m_J_Contact_final_far_fwd() {
        return run('Contact', 3200, new List<String>{
            'Agvz/Agvm/std',
            'Gwoq/Gwop/std',
            'Zpcyz/Zpcym/std',
            'Skuyq/Skuyp/std',
            'Bbrizz/Bbrizm/std',
            'Nheuoq/Nheuop/std',
            'Quloluz/Qulolum/std',
            'Uxlvgvq/Uxlvgvp/std',
            'Vcsmzkiz/Vcsmzkim/std',
            'Kfjmwzwq/Kfjmwzwp/std'
        });
    }
    public static String m_J_Contact_final_far_rev() {
        return run('Contact', 3250, new List<String>{
            'Agvm/Agvz/std',
            'Gwop/Gwoq/std',
            'Zpcym/Zpcyz/std',
            'Skuyp/Skuyq/std',
            'Bbrizm/Bbrizz/std',
            'Nheuop/Nheuoq/std',
            'Qulolum/Quloluz/std',
            'Uxlvgvp/Uxlvgvq/std',
            'Vcsmzkim/Vcsmzkiz/std',
            'Kfjmwzwp/Kfjmwzwq/std'
        });
    }
    public static String m_J_Lead_final_far_fwd() {
        return run('Lead', 3300, new List<String>{
            'Agvz/Agvm/std',
            'Gwoq/Gwop/std',
            'Zpcyz/Zpcym/std',
            'Skuyq/Skuyp/std',
            'Bbrizz/Bbrizm/std',
            'Nheuoq/Nheuop/std',
            'Quloluz/Qulolum/std',
            'Uxlvgvq/Uxlvgvp/std',
            'Vcsmzkiz/Vcsmzkim/std',
            'Kfjmwzwq/Kfjmwzwp/std'
        });
    }
    public static String m_J_Lead_final_far_rev() {
        return run('Lead', 3350, new List<String>{
            'Agvm/Agvz/std',
            'Gwop/Gwoq/std',
            'Zpcym/Zpcyz/std',
            'Skuyp/Skuyq/std',
            'Bbrizm/Bbrizz/std',
            'Nheuop/Nheuoq/std',
            'Qulolum/Quloluz/std',
            'Uxlvgvp/Uxlvgvq/std',
            'Vcsmzkim/Vcsmzkiz/std',
            'Kfjmwzwp/Kfjmwzwq/std'
        });
    }
    public static String m_K_Contact_noemail_fwd() {
        return run('Contact', 3400, new List<String>{
            'Mtpu/Ntpu/noemail',
            'Qvmlg/Avmlg/noemail',
            'Wstdbr/Dstdbr/noemail',
            'Tpmxi/Tpnxi/noemail',
            'Gxgqoh/Gxgaoh/noemail',
            'Mnnwfsy/Mnndfsy/noemail',
            'Yozjtm/Yozjtn/noemail',
            'Wfdstsq/Wfdstsa/noemail',
            'Hyfwvrfw/Hyfwvrfd/noemail'
        });
    }
    public static String m_K_Contact_noemail_rev() {
        return run('Contact', 3450, new List<String>{
            'Ntpu/Mtpu/noemail',
            'Avmlg/Qvmlg/noemail',
            'Dstdbr/Wstdbr/noemail',
            'Tpnxi/Tpmxi/noemail',
            'Gxgaoh/Gxgqoh/noemail',
            'Mnndfsy/Mnnwfsy/noemail',
            'Yozjtn/Yozjtm/noemail',
            'Wfdstsa/Wfdstsq/noemail',
            'Hyfwvrfd/Hyfwvrfw/noemail'
        });
    }
    public static String m_K_Lead_noemail_fwd() {
        return run('Lead', 3500, new List<String>{
            'Mtpu/Ntpu/noemail',
            'Qvmlg/Avmlg/noemail',
            'Wstdbr/Dstdbr/noemail',
            'Tpmxi/Tpnxi/noemail',
            'Gxgqoh/Gxgaoh/noemail',
            'Mnnwfsy/Mnndfsy/noemail',
            'Yozjtm/Yozjtn/noemail',
            'Wfdstsq/Wfdstsa/noemail',
            'Hyfwvrfw/Hyfwvrfd/noemail'
        });
    }
    public static String m_K_Lead_noemail_rev() {
        return run('Lead', 3550, new List<String>{
            'Ntpu/Mtpu/noemail',
            'Avmlg/Qvmlg/noemail',
            'Dstdbr/Wstdbr/noemail',
            'Tpnxi/Tpmxi/noemail',
            'Gxgaoh/Gxgqoh/noemail',
            'Mnndfsy/Mnnwfsy/noemail',
            'Yozjtn/Yozjtm/noemail',
            'Wfdstsa/Wfdstsq/noemail',
            'Hyfwvrfd/Hyfwvrfw/noemail'
        });
    }
    public static String m_K_Contact_diffemail_fwd() {
        return run('Contact', 3600, new List<String>{
            'Mtpu/Ntpu/diffemail',
            'Qvmlg/Avmlg/diffemail',
            'Wstdbr/Dstdbr/diffemail',
            'Tpmxi/Tpnxi/diffemail',
            'Gxgqoh/Gxgaoh/diffemail',
            'Mnnwfsy/Mnndfsy/diffemail',
            'Yozjtm/Yozjtn/diffemail',
            'Wfdstsq/Wfdstsa/diffemail',
            'Hyfwvrfw/Hyfwvrfd/diffemail'
        });
    }
    public static String m_K_Contact_diffemail_rev() {
        return run('Contact', 3650, new List<String>{
            'Ntpu/Mtpu/diffemail',
            'Avmlg/Qvmlg/diffemail',
            'Dstdbr/Wstdbr/diffemail',
            'Tpnxi/Tpmxi/diffemail',
            'Gxgaoh/Gxgqoh/diffemail',
            'Mnndfsy/Mnnwfsy/diffemail',
            'Yozjtn/Yozjtm/diffemail',
            'Wfdstsa/Wfdstsq/diffemail',
            'Hyfwvrfd/Hyfwvrfw/diffemail'
        });
    }
    public static String m_K_Lead_diffemail_fwd() {
        return run('Lead', 3700, new List<String>{
            'Mtpu/Ntpu/diffemail',
            'Qvmlg/Avmlg/diffemail',
            'Wstdbr/Dstdbr/diffemail',
            'Tpmxi/Tpnxi/diffemail',
            'Gxgqoh/Gxgaoh/diffemail',
            'Mnnwfsy/Mnndfsy/diffemail',
            'Yozjtm/Yozjtn/diffemail',
            'Wfdstsq/Wfdstsa/diffemail',
            'Hyfwvrfw/Hyfwvrfd/diffemail'
        });
    }
    public static String m_K_Lead_diffemail_rev() {
        return run('Lead', 3750, new List<String>{
            'Ntpu/Mtpu/diffemail',
            'Avmlg/Qvmlg/diffemail',
            'Dstdbr/Wstdbr/diffemail',
            'Tpnxi/Tpmxi/diffemail',
            'Gxgaoh/Gxgqoh/diffemail',
            'Mnndfsy/Mnnwfsy/diffemail',
            'Yozjtn/Yozjtm/diffemail',
            'Wfdstsa/Wfdstsq/diffemail',
            'Hyfwvrfd/Hyfwvrfw/diffemail'
        });
    }
    public static String m_K_Contact_diffname_fwd() {
        return run('Contact', 3800, new List<String>{
            'Mtpu/Ntpu/diffname',
            'Qvmlg/Avmlg/diffname',
            'Wstdbr/Dstdbr/diffname',
            'Tpmxi/Tpnxi/diffname',
            'Gxgqoh/Gxgaoh/diffname',
            'Mnnwfsy/Mnndfsy/diffname',
            'Yozjtm/Yozjtn/diffname',
            'Wfdstsq/Wfdstsa/diffname',
            'Hyfwvrfw/Hyfwvrfd/diffname'
        });
    }
    public static String m_K_Contact_diffname_rev() {
        return run('Contact', 3850, new List<String>{
            'Ntpu/Mtpu/diffname',
            'Avmlg/Qvmlg/diffname',
            'Dstdbr/Wstdbr/diffname',
            'Tpnxi/Tpmxi/diffname',
            'Gxgaoh/Gxgqoh/diffname',
            'Mnndfsy/Mnnwfsy/diffname',
            'Yozjtn/Yozjtm/diffname',
            'Wfdstsa/Wfdstsq/diffname',
            'Hyfwvrfd/Hyfwvrfw/diffname'
        });
    }
    public static String m_K_Lead_diffname_fwd() {
        return run('Lead', 3900, new List<String>{
            'Mtpu/Ntpu/diffname',
            'Qvmlg/Avmlg/diffname',
            'Wstdbr/Dstdbr/diffname',
            'Tpmxi/Tpnxi/diffname',
            'Gxgqoh/Gxgaoh/diffname',
            'Mnnwfsy/Mnndfsy/diffname',
            'Yozjtm/Yozjtn/diffname',
            'Wfdstsq/Wfdstsa/diffname',
            'Hyfwvrfw/Hyfwvrfd/diffname'
        });
    }
    public static String m_K_Lead_diffname_rev() {
        return run('Lead', 3950, new List<String>{
            'Ntpu/Mtpu/diffname',
            'Avmlg/Qvmlg/diffname',
            'Dstdbr/Wstdbr/diffname',
            'Tpnxi/Tpmxi/diffname',
            'Gxgaoh/Gxgqoh/diffname',
            'Mnndfsy/Mnnwfsy/diffname',
            'Yozjtn/Yozjtm/diffname',
            'Wfdstsa/Wfdstsq/diffname',
            'Hyfwvrfd/Hyfwvrfw/diffname'
        });
    }
    public static String m_K_Contact_phone_fwd() {
        return run('Contact', 4000, new List<String>{
            'Mtpu/Ntpu/phone',
            'Qvmlg/Avmlg/phone',
            'Wstdbr/Dstdbr/phone',
            'Tpmxi/Tpnxi/phone',
            'Gxgqoh/Gxgaoh/phone',
            'Mnnwfsy/Mnndfsy/phone',
            'Yozjtm/Yozjtn/phone',
            'Wfdstsq/Wfdstsa/phone',
            'Hyfwvrfw/Hyfwvrfd/phone'
        });
    }
    public static String m_K_Contact_phone_rev() {
        return run('Contact', 4050, new List<String>{
            'Ntpu/Mtpu/phone',
            'Avmlg/Qvmlg/phone',
            'Dstdbr/Wstdbr/phone',
            'Tpnxi/Tpmxi/phone',
            'Gxgaoh/Gxgqoh/phone',
            'Mnndfsy/Mnnwfsy/phone',
            'Yozjtn/Yozjtm/phone',
            'Wfdstsa/Wfdstsq/phone',
            'Hyfwvrfd/Hyfwvrfw/phone'
        });
    }
    public static String m_K_Lead_phone_fwd() {
        return run('Lead', 4100, new List<String>{
            'Mtpu/Ntpu/phone',
            'Qvmlg/Avmlg/phone',
            'Wstdbr/Dstdbr/phone',
            'Tpmxi/Tpnxi/phone',
            'Gxgqoh/Gxgaoh/phone',
            'Mnnwfsy/Mnndfsy/phone',
            'Yozjtm/Yozjtn/phone',
            'Wfdstsq/Wfdstsa/phone',
            'Hyfwvrfw/Hyfwvrfd/phone'
        });
    }
    public static String m_K_Lead_phone_rev() {
        return run('Lead', 4150, new List<String>{
            'Ntpu/Mtpu/phone',
            'Avmlg/Qvmlg/phone',
            'Dstdbr/Wstdbr/phone',
            'Tpnxi/Tpmxi/phone',
            'Gxgaoh/Gxgqoh/phone',
            'Mnndfsy/Mnnwfsy/phone',
            'Yozjtn/Yozjtm/phone',
            'Wfdstsa/Wfdstsq/phone',
            'Hyfwvrfd/Hyfwvrfw/phone'
        });
    }
}
