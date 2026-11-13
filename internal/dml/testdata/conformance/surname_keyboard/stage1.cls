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
    public static String m_A_adjacent_fwd() {
        return run('Contact', 1000, new List<String>{
            'Riim/Riimn/std',
            'Ikir/Ikirt/std',
            'Sznc/Szncv/std',
            'Ssxb/Ssxbn/std',
            'Yent/Yenty/std',
            'Zela/Zelak/std',
            'Rume/Rumer/std',
            'Jlun/Jlunm/std'
        });
    }
    public static String m_A_adjacent_rev() {
        return run('Contact', 1050, new List<String>{
            'Riimn/Riim/std',
            'Ikirt/Ikir/std',
            'Szncv/Sznc/std',
            'Ssxbn/Ssxb/std',
            'Yenty/Yent/std',
            'Zelak/Zela/std',
            'Rumer/Rume/std',
            'Jlunm/Jlun/std'
        });
    }
    public static String m_A_column_fwd() {
        return run('Contact', 1100, new List<String>{
            'Riim/Riimj/std',
            'Ikir/Ikirf/std',
            'Sznc/Szncd/std',
            'Ssxb/Ssxbg/std',
            'Yent/Yentg/std',
            'Zela/Zelaq/std',
            'Rume/Rumed/std',
            'Jlun/Jlunh/std'
        });
    }
    public static String m_A_column_rev() {
        return run('Contact', 1150, new List<String>{
            'Riimj/Riim/std',
            'Ikirf/Ikir/std',
            'Szncd/Sznc/std',
            'Ssxbg/Ssxb/std',
            'Yentg/Yent/std',
            'Zelaq/Zela/std',
            'Rumed/Rume/std',
            'Jlunh/Jlun/std'
        });
    }
    public static String m_A_far_fwd() {
        return run('Contact', 1200, new List<String>{
            'Riim/Riimq/std',
            'Ikir/Ikiro/std',
            'Sznc/Sznco/std',
            'Ssxb/Ssxbo/std',
            'Yent/Yento/std',
            'Zela/Zelao/std',
            'Rume/Rumeo/std',
            'Jlun/Jlunq/std'
        });
    }
    public static String m_A_far_rev() {
        return run('Contact', 1250, new List<String>{
            'Riimq/Riim/std',
            'Ikiro/Ikir/std',
            'Sznco/Sznc/std',
            'Ssxbo/Ssxb/std',
            'Yento/Yent/std',
            'Zelao/Zela/std',
            'Rumeo/Rume/std',
            'Jlunq/Jlun/std'
        });
    }
    public static String m_A_vowel_fwd() {
        return run('Contact', 1300, new List<String>{
            'Riim/Riima/std',
            'Ikir/Ikira/std',
            'Sznc/Sznca/std',
            'Ssxb/Ssxba/std',
            'Yent/Yenta/std',
            'Zela/Zelae/std',
            'Rume/Rumea/std',
            'Jlun/Jluna/std'
        });
    }
    public static String m_A_vowel_rev() {
        return run('Contact', 1350, new List<String>{
            'Riima/Riim/std',
            'Ikira/Ikir/std',
            'Sznca/Sznc/std',
            'Ssxba/Ssxb/std',
            'Yenta/Yent/std',
            'Zelae/Zela/std',
            'Rumea/Rume/std',
            'Jluna/Jlun/std'
        });
    }
    public static String m_A_doubled_fwd() {
        return run('Contact', 1400, new List<String>{
            'Riim/Riimm/std',
            'Ikir/Ikirr/std',
            'Sznc/Szncc/std',
            'Ssxb/Ssxbb/std',
            'Yent/Yentt/std',
            'Zela/Zelaa/std',
            'Rume/Rumee/std',
            'Jlun/Jlunn/std'
        });
    }
    public static String m_A_doubled_rev() {
        return run('Contact', 1450, new List<String>{
            'Riimm/Riim/std',
            'Ikirr/Ikir/std',
            'Szncc/Sznc/std',
            'Ssxbb/Ssxb/std',
            'Yentt/Yent/std',
            'Zelaa/Zela/std',
            'Rumee/Rume/std',
            'Jlunn/Jlun/std'
        });
    }
    public static String m_A_s_fwd() {
        return run('Contact', 1500, new List<String>{
            'Riim/Riims/std',
            'Ikir/Ikirs/std',
            'Sznc/Szncs/std',
            'Ssxb/Ssxbs/std',
            'Kuhj/Kuhjs/std',
            'Lunf/Lunfs/std',
            'Icgf/Icgfs/std',
            'Braf/Brafs/std',
            'Pzdf/Pzdfs/std',
            'Gzzc/Gzzcs/std',
            'Zzxf/Zzxfs/std',
            'Czzx/Czzxs/std',
            'Yent/Yents/std',
            'Zela/Zelas/std',
            'Rume/Rumes/std',
            'Jlun/Jluns/std'
        });
    }
    public static String m_A_s_rev() {
        return run('Contact', 1550, new List<String>{
            'Riims/Riim/std',
            'Ikirs/Ikir/std',
            'Szncs/Sznc/std',
            'Ssxbs/Ssxb/std',
            'Kuhjs/Kuhj/std',
            'Lunfs/Lunf/std',
            'Icgfs/Icgf/std',
            'Brafs/Braf/std',
            'Pzdfs/Pzdf/std',
            'Gzzcs/Gzzc/std',
            'Zzxfs/Zzxf/std',
            'Czzxs/Czzx/std',
            'Yents/Yent/std',
            'Zelas/Zela/std',
            'Rumes/Rume/std',
            'Jluns/Jlun/std'
        });
    }
    public static String m_A_p_fwd() {
        return run('Contact', 1600, new List<String>{
            'Riim/Riimp/std',
            'Ikir/Ikirp/std',
            'Sznc/Szncp/std',
            'Ssxb/Ssxbp/std',
            'Kuhj/Kuhjp/std',
            'Lunf/Lunfp/std',
            'Icgf/Icgfp/std',
            'Braf/Brafp/std',
            'Pzdf/Pzdfp/std',
            'Gzzc/Gzzcp/std',
            'Zzxf/Zzxfp/std',
            'Czzx/Czzxp/std',
            'Yent/Yentp/std',
            'Zela/Zelap/std',
            'Rume/Rumep/std',
            'Jlun/Jlunp/std'
        });
    }
    public static String m_A_p_rev() {
        return run('Contact', 1650, new List<String>{
            'Riimp/Riim/std',
            'Ikirp/Ikir/std',
            'Szncp/Sznc/std',
            'Ssxbp/Ssxb/std',
            'Kuhjp/Kuhj/std',
            'Lunfp/Lunf/std',
            'Icgfp/Icgf/std',
            'Brafp/Braf/std',
            'Pzdfp/Pzdf/std',
            'Gzzcp/Gzzc/std',
            'Zzxfp/Zzxf/std',
            'Czzxp/Czzx/std',
            'Yentp/Yent/std',
            'Zelap/Zela/std',
            'Rumep/Rume/std',
            'Jlunp/Jlun/std'
        });
    }
    public static String m_A_z_fwd() {
        return run('Contact', 1700, new List<String>{
            'Riim/Riimz/std',
            'Ikir/Ikirz/std',
            'Sznc/Szncz/std',
            'Ssxb/Ssxbz/std',
            'Yent/Yentz/std',
            'Zela/Zelaz/std',
            'Rume/Rumez/std',
            'Jlun/Jlunz/std'
        });
    }
    public static String m_A_z_rev() {
        return run('Contact', 1750, new List<String>{
            'Riimz/Riim/std',
            'Ikirz/Ikir/std',
            'Szncz/Sznc/std',
            'Ssxbz/Ssxb/std',
            'Yentz/Yent/std',
            'Zelaz/Zela/std',
            'Rumez/Rume/std',
            'Jlunz/Jlun/std'
        });
    }
    public static String m_B_fwd() {
        return run('Contact', 1800, new List<String>{
            'Fhp/Fhpa/std',
            'Ocj/Ocja/std',
            'Ngi/Ngia/std',
            'Xkk/Xkka/std',
            'Bpg/Bpga/std',
            'Dds/Ddsa/std',
            'Igrn/Igrna/std',
            'Xeuu/Xeuua/std',
            'Wflr/Wflra/std',
            'Gkhj/Gkhja/std',
            'Prnd/Prnda/std',
            'Cxcb/Cxcba/std'
        });
    }
    public static String m_LeadB_fwd() {
        return run('Lead', 1850, new List<String>{
            'Fhp/Fhpa/std',
            'Ocj/Ocja/std',
            'Ngi/Ngia/std',
            'Xkk/Xkka/std',
            'Bpg/Bpga/std',
            'Dds/Ddsa/std',
            'Igrn/Igrna/std',
            'Xeuu/Xeuua/std',
            'Wflr/Wflra/std',
            'Gkhj/Gkhja/std',
            'Prnd/Prnda/std',
            'Cxcb/Cxcba/std'
        });
    }
    public static String m_B_rev() {
        return run('Contact', 1900, new List<String>{
            'Fhpa/Fhp/std',
            'Ocja/Ocj/std',
            'Ngia/Ngi/std',
            'Xkka/Xkk/std',
            'Bpga/Bpg/std',
            'Ddsa/Dds/std',
            'Igrna/Igrn/std',
            'Xeuua/Xeuu/std',
            'Wflra/Wflr/std',
            'Gkhja/Gkhj/std',
            'Prnda/Prnd/std',
            'Cxcba/Cxcb/std'
        });
    }
    public static String m_LeadB_rev() {
        return run('Lead', 1950, new List<String>{
            'Fhpa/Fhp/std',
            'Ocja/Ocj/std',
            'Ngia/Ngi/std',
            'Xkka/Xkk/std',
            'Bpga/Bpg/std',
            'Ddsa/Dds/std',
            'Igrna/Igrn/std',
            'Xeuua/Xeuu/std',
            'Wflra/Wflr/std',
            'Gkhja/Gkhj/std',
            'Prnda/Prnd/std',
            'Cxcba/Cxcb/std'
        });
    }
    public static String m_C_t1_fwd() {
        return run('Contact', 2000, new List<String>{
            'Uhv/Uhvs/std',
            'Cpv/Cpvs/std',
            'Cnh/Cnhs/std',
            'Oyr/Oyrs/std',
            'Bcw/Bcws/std',
            'Erj/Erjs/std',
            'Zmj/Zmjs/std',
            'Evj/Evjs/std',
            'Vvh/Vvhs/std',
            'Nox/Noxs/std',
            'Hfx/Hfxs/std',
            'Jgr/Jgrs/std'
        });
    }
    public static String m_C_t1_rev() {
        return run('Contact', 2050, new List<String>{
            'Uhvs/Uhv/std',
            'Cpvs/Cpv/std',
            'Cnhs/Cnh/std',
            'Oyrs/Oyr/std',
            'Bcws/Bcw/std',
            'Erjs/Erj/std',
            'Zmjs/Zmj/std',
            'Evjs/Evj/std',
            'Vvhs/Vvh/std',
            'Noxs/Nox/std',
            'Hfxs/Hfx/std',
            'Jgrs/Jgr/std'
        });
    }
    public static String m_C_t2_fwd() {
        return run('Contact', 2100, new List<String>{
            'Uhv/Uhvp/std',
            'Cpv/Cpvp/std',
            'Cnh/Cnhp/std',
            'Oyr/Oyrp/std',
            'Bcw/Bcwp/std',
            'Erj/Erjp/std',
            'Zmj/Zmjp/std',
            'Evj/Evjp/std',
            'Vvh/Vvhp/std',
            'Nox/Noxp/std',
            'Hfx/Hfxp/std',
            'Jgr/Jgrp/std'
        });
    }
    public static String m_C_t2_rev() {
        return run('Contact', 2150, new List<String>{
            'Uhvp/Uhv/std',
            'Cpvp/Cpv/std',
            'Cnhp/Cnh/std',
            'Oyrp/Oyr/std',
            'Bcwp/Bcw/std',
            'Erjp/Erj/std',
            'Zmjp/Zmj/std',
            'Evjp/Evj/std',
            'Vvhp/Vvh/std',
            'Noxp/Nox/std',
            'Hfxp/Hfx/std',
            'Jgrp/Jgr/std'
        });
    }
    public static String m_C_t3_fwd() {
        return run('Contact', 2200, new List<String>{
            'Uhv/Uhvk/std',
            'Cpv/Cpvk/std',
            'Cnh/Cnhk/std',
            'Oyr/Oyrk/std',
            'Bcw/Bcwk/std',
            'Erj/Erjk/std',
            'Zmj/Zmjk/std',
            'Evj/Evjk/std',
            'Vvh/Vvhk/std',
            'Nox/Noxk/std',
            'Hfx/Hfxk/std',
            'Jgr/Jgrk/std'
        });
    }
    public static String m_C_t3_rev() {
        return run('Contact', 2250, new List<String>{
            'Uhvk/Uhv/std',
            'Cpvk/Cpv/std',
            'Cnhk/Cnh/std',
            'Oyrk/Oyr/std',
            'Bcwk/Bcw/std',
            'Erjk/Erj/std',
            'Zmjk/Zmj/std',
            'Evjk/Evj/std',
            'Vvhk/Vvh/std',
            'Noxk/Nox/std',
            'Hfxk/Hfx/std',
            'Jgrk/Jgr/std'
        });
    }
    public static String m_D1_fwd() {
        return run('Contact', 2300, new List<String>{
            'Xjm/Xjn/std',
            'Lpm/Lpn/std',
            'Uikm/Uikn/std',
            'Xkhm/Xkhn/std',
            'Pszkm/Pszkn/std',
            'Pslsm/Pslsn/std',
            'Uawhgm/Uawhgn/std',
            'Jhrxpm/Jhrxpn/std',
            'Ftjsfrm/Ftjsfrn/std',
            'Vrdtjkm/Vrdtjkn/std',
            'Hvvzswxm/Hvvzswxn/std',
            'Zxltkmnm/Zxltkmnn/std',
            'Flrym/Flryn/std',
            'Tmxrm/Tmxrn/std',
            'Vpwym/Vpwyn/std',
            'Tmvum/Tmvun/std',
            'Vyckb/Vyckv/std'
        });
    }
    public static String m_LeadD1_fwd() {
        return run('Lead', 2350, new List<String>{
            'Xjm/Xjn/std',
            'Lpm/Lpn/std',
            'Uikm/Uikn/std',
            'Xkhm/Xkhn/std',
            'Pszkm/Pszkn/std',
            'Pslsm/Pslsn/std',
            'Uawhgm/Uawhgn/std',
            'Jhrxpm/Jhrxpn/std',
            'Ftjsfrm/Ftjsfrn/std',
            'Vrdtjkm/Vrdtjkn/std',
            'Hvvzswxm/Hvvzswxn/std',
            'Zxltkmnm/Zxltkmnn/std',
            'Flrym/Flryn/std',
            'Tmxrm/Tmxrn/std',
            'Vpwym/Vpwyn/std',
            'Tmvum/Tmvun/std',
            'Vyckb/Vyckv/std'
        });
    }
    public static String m_D2_fwd() {
        return run('Contact', 2400, new List<String>{
            'Eckpb/Eckpv/std',
            'Ydcxc/Ydcxv/std',
            'Mguyc/Mguyv/std',
            'Nutvs/Nutvd/std',
            'Mppas/Mppad/std',
            'Kinzi/Kinzo/std',
            'Srpti/Srpto/std',
            'Vegrk/Vegrl/std',
            'Flvok/Flvol/std',
            'Kcvhq/Kcvhp/std',
            'Ttjmq/Ttjmp/std',
            'Shslq/Shslp/std',
            'Dampq/Dampp/std',
            'Jjhez/Jjhem/std',
            'Parxz/Parxm/std',
            'Kytwz/Kytwm/std',
            'Vhssz/Vhssm/std'
        });
    }
    public static String m_LeadD2_fwd() {
        return run('Lead', 2450, new List<String>{
            'Eckpb/Eckpv/std',
            'Ydcxc/Ydcxv/std',
            'Mguyc/Mguyv/std',
            'Nutvs/Nutvd/std',
            'Mppas/Mppad/std',
            'Kinzi/Kinzo/std',
            'Srpti/Srpto/std',
            'Vegrk/Vegrl/std',
            'Flvok/Flvol/std',
            'Kcvhq/Kcvhp/std',
            'Ttjmq/Ttjmp/std',
            'Shslq/Shslp/std',
            'Dampq/Dampp/std',
            'Jjhez/Jjhem/std',
            'Parxz/Parxm/std',
            'Kytwz/Kytwm/std',
            'Vhssz/Vhssm/std'
        });
    }
    public static String m_D1_rev() {
        return run('Contact', 2500, new List<String>{
            'Xjn/Xjm/std',
            'Lpn/Lpm/std',
            'Uikn/Uikm/std',
            'Xkhn/Xkhm/std',
            'Pszkn/Pszkm/std',
            'Pslsn/Pslsm/std',
            'Uawhgn/Uawhgm/std',
            'Jhrxpn/Jhrxpm/std',
            'Ftjsfrn/Ftjsfrm/std',
            'Vrdtjkn/Vrdtjkm/std',
            'Hvvzswxn/Hvvzswxm/std',
            'Zxltkmnn/Zxltkmnm/std',
            'Flryn/Flrym/std',
            'Tmxrn/Tmxrm/std',
            'Vpwyn/Vpwym/std',
            'Tmvun/Tmvum/std',
            'Vyckv/Vyckb/std'
        });
    }
    public static String m_LeadD1_rev() {
        return run('Lead', 2550, new List<String>{
            'Xjn/Xjm/std',
            'Lpn/Lpm/std',
            'Uikn/Uikm/std',
            'Xkhn/Xkhm/std',
            'Pszkn/Pszkm/std',
            'Pslsn/Pslsm/std',
            'Uawhgn/Uawhgm/std',
            'Jhrxpn/Jhrxpm/std',
            'Ftjsfrn/Ftjsfrm/std',
            'Vrdtjkn/Vrdtjkm/std',
            'Hvvzswxn/Hvvzswxm/std',
            'Zxltkmnn/Zxltkmnm/std',
            'Flryn/Flrym/std',
            'Tmxrn/Tmxrm/std',
            'Vpwyn/Vpwym/std',
            'Tmvun/Tmvum/std',
            'Vyckv/Vyckb/std'
        });
    }
    public static String m_D2_rev() {
        return run('Contact', 2600, new List<String>{
            'Eckpv/Eckpb/std',
            'Ydcxv/Ydcxc/std',
            'Mguyv/Mguyc/std',
            'Nutvd/Nutvs/std',
            'Mppad/Mppas/std',
            'Kinzo/Kinzi/std',
            'Srpto/Srpti/std',
            'Vegrl/Vegrk/std',
            'Flvol/Flvok/std',
            'Kcvhp/Kcvhq/std',
            'Ttjmp/Ttjmq/std',
            'Shslp/Shslq/std',
            'Dampp/Dampq/std',
            'Jjhem/Jjhez/std',
            'Parxm/Parxz/std',
            'Kytwm/Kytwz/std',
            'Vhssm/Vhssz/std'
        });
    }
    public static String m_LeadD2_rev() {
        return run('Lead', 2650, new List<String>{
            'Eckpv/Eckpb/std',
            'Ydcxv/Ydcxc/std',
            'Mguyv/Mguyc/std',
            'Nutvd/Nutvs/std',
            'Mppad/Mppas/std',
            'Kinzo/Kinzi/std',
            'Srpto/Srpti/std',
            'Vegrl/Vegrk/std',
            'Flvol/Flvok/std',
            'Kcvhp/Kcvhq/std',
            'Ttjmp/Ttjmq/std',
            'Shslp/Shslq/std',
            'Dampp/Dampq/std',
            'Jjhem/Jjhez/std',
            'Parxm/Parxz/std',
            'Kytwm/Kytwz/std',
            'Vhssm/Vhssz/std'
        });
    }
    public static String m_E_fwd() {
        return run('Contact', 2700, new List<String>{
            'Ykj/Yjk/std',
            'Ntr/Nrt/std',
            'Jdf/Jfd/std',
            'Dbn/Dnb/std',
            'Cbn/Cnb/std',
            'Zbn/Znb/std',
            'Rxc/Rcx/std',
            'Bxc/Bcx/std',
            'Zvb/Zbv/std',
            'Vcx/Vxc/std',
            'Lzx/Lxz/std',
            'Fxz/Fzx/std',
            'Xxc/Xcx/std',
            'Xzx/Xxz/std'
        });
    }
    public static String m_E_rev() {
        return run('Contact', 2750, new List<String>{
            'Yjk/Ykj/std',
            'Nrt/Ntr/std',
            'Jfd/Jdf/std',
            'Dnb/Dbn/std',
            'Cnb/Cbn/std',
            'Znb/Zbn/std',
            'Rcx/Rxc/std',
            'Bcx/Bxc/std',
            'Zbv/Zvb/std',
            'Vxc/Vcx/std',
            'Lxz/Lzx/std',
            'Fzx/Fxz/std',
            'Xcx/Xxc/std',
            'Xxz/Xzx/std'
        });
    }
    public static String m_F_fwd() {
        return run('Contact', 2800, new List<String>{
            'Hnhyt/Hnthy/std',
            'Ytgtr/Ytrgt/std',
            'Jtthg/Jtgth/std',
            'Hnthg/Hnhgt/std',
            'Nnnbh/Nnbhn/std',
            'Jzgth/Jzthg/std',
            'Wftgf/Wfftg/std',
            'Yzgnb/Yznbg/std',
            'Cftfg/Cffgt/std',
            'Pzghn/Pzngh/std',
            'Lysqw/Lyqws/std',
            'Hpdxs/Hpxsd/std',
            'Qzcde/Qzdec/std',
            'Rbtgb/Rbbtg/std',
            'Bsfvr/Bsrfv/std',
            'Zrgbt/Zrbtg/std',
            'Lwxvz/Lwzxv/std',
            'Kajgf/Kagfj/std',
            'Gqxgb/Gqbxg/std',
            'Wcbwf/Wcfbw/std'
        });
    }
    public static String m_F_rev() {
        return run('Contact', 2850, new List<String>{
            'Hnthy/Hnhyt/std',
            'Ytrgt/Ytgtr/std',
            'Jtgth/Jtthg/std',
            'Hnhgt/Hnthg/std',
            'Nnbhn/Nnnbh/std',
            'Jzthg/Jzgth/std',
            'Wfftg/Wftgf/std',
            'Yznbg/Yzgnb/std',
            'Cffgt/Cftfg/std',
            'Pzngh/Pzghn/std',
            'Lyqws/Lysqw/std',
            'Hpxsd/Hpdxs/std',
            'Qzdec/Qzcde/std',
            'Rbbtg/Rbtgb/std',
            'Bsrfv/Bsfvr/std',
            'Zrbtg/Zrgbt/std',
            'Lwzxv/Lwxvz/std',
            'Kagfj/Kajgf/std',
            'Gqbxg/Gqxgb/std',
            'Wcfbw/Wcbwf/std'
        });
    }
    public static String m_G1_fwd() {
        return run('Contact', 2900, new List<String>{
            'Qhblhytp/Qhblhypt/std',
            'Qyrktkrl/Qyrktklr/std',
            'Jhlflkgp/Jhlflkpg/std',
            'Nrnjhllv/Nrnjhlvl/std',
            'Gklnjdrl/Gklnjdlr/std',
            'Fqlyymkd/Fqlyymdk/std',
            'Nfpkkllr/Nfpkklrl/std',
            'Dyrpvyms/Dyrpvysm/std',
            'Zdrrygtp/Zdrrygpt/std',
            'Tklnhslv/Tklnhsvl/std',
            'Hqjhnzlr/Hqjhnzrl/std'
        });
    }
    public static String m_G2_fwd() {
        return run('Contact', 2950, new List<String>{
            'Lcegrhdk/Lcegrhkd/std',
            'Babetklr/Babetkrl/std',
            'Qhmcmyzh/Qhmcmyhz/std',
            'Wstlhbsm/Wstlhbms/std',
            'Gsldggfl/Gsldgglf/std',
            'Cltnmxfl/Cltnmxlf/std',
            'Lznhjwxj/Lznhjwjx/std',
            'Bgwdbbxj/Bgwdbbjx/std',
            'Qweevgrl/Qweevglr/std',
            'Bndjgzmx/Bndjgzxm/std',
            'Lllcvwrl/Lllcvwlr/std'
        });
    }
    public static String m_G1_rev() {
        return run('Contact', 3000, new List<String>{
            'Qhblhypt/Qhblhytp/std',
            'Qyrktklr/Qyrktkrl/std',
            'Jhlflkpg/Jhlflkgp/std',
            'Nrnjhlvl/Nrnjhllv/std',
            'Gklnjdlr/Gklnjdrl/std',
            'Fqlyymdk/Fqlyymkd/std',
            'Nfpkklrl/Nfpkkllr/std',
            'Dyrpvysm/Dyrpvyms/std',
            'Zdrrygpt/Zdrrygtp/std',
            'Tklnhsvl/Tklnhslv/std',
            'Hqjhnzrl/Hqjhnzlr/std'
        });
    }
    public static String m_G2_rev() {
        return run('Contact', 3050, new List<String>{
            'Lcegrhkd/Lcegrhdk/std',
            'Babetkrl/Babetklr/std',
            'Qhmcmyhz/Qhmcmyzh/std',
            'Wstlhbms/Wstlhbsm/std',
            'Gsldgglf/Gsldggfl/std',
            'Cltnmxlf/Cltnmxfl/std',
            'Lznhjwjx/Lznhjwxj/std',
            'Bgwdbbjx/Bgwdbbxj/std',
            'Qweevglr/Qweevgrl/std',
            'Bndjgzxm/Bndjgzmx/std',
            'Lllcvwlr/Lllcvwrl/std'
        });
    }
}
