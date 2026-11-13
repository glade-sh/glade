public class AccountBehavior {
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
    private static String nz(String s) { return s == '_' ? null : s; }
    private static Account mk(String name, String web, String street, String city, String zip, String phone) {
        return new Account(Name = name, Website = nz(web), BillingStreet = nz(street), BillingCity = nz(city), BillingPostalCode = nz(zip), Phone = nz(phone));
    }
    public static String run(List<String> specs) {
        String ruleName = 'Standard_Account_Duplicate_Rule';
        List<String> all = new List<String>(specs);
        all.add('Zorbexa Tanagro~_~_~Ctpos~_~_~Zorbexa Tanagro~_~_~Ctpos~_~_~insert');
        all.add('Wivelsk Marnotte~_~_~Ctneg~_~_~Plorind Quavasque~_~_~Ctneg~_~_~insert');
        List<Account> seeds = new List<Account>(), cands = new List<Account>(), edits = new List<Account>();
        List<String> ops = new List<String>();
        for (Integer i = 0; i < all.size(); i++) {
            List<String> p = all[i].split('~');
            seeds.add(mk(p[0], p[1], p[2], p[3], p[4], p[5]));
            cands.add(mk(p[6], p[7], p[8], p[9], p[10], p[11]));
            ops.add(p[12]);
            if (p[12] != 'insert') edits.add(new Account(Name = 'Edit' + String.valueOf(i) + 'Zq', Website = 'https://edit' + String.valueOf(i) + '.invalid', BillingPostalCode = '9' + String.valueOf(1000 + i)));
        }
        insert seeds;
        if (!edits.isEmpty()) insert edits;
        Map<Id,String> labels = new Map<Id,String>();
        for (Integer i = 0; i < seeds.size(); i++) labels.put(seeds[i].Id, String.valueOf(i));
        List<Datacloud.FindDuplicatesResult> found = Datacloud.FindDuplicates.findDuplicates(cands);
        System.assertEquals(all.size(), found.size(), 'incomplete Find response');
        List<Database.SaveResult> saved = new List<Database.SaveResult>();
        Integer ei = 0;
        List<Account> insertList = new List<Account>();
        Map<Integer,Integer> idx = new Map<Integer,Integer>();
        Database.SaveResult[] insertResults = new Database.SaveResult[0];
        List<Integer> insertIdx = new List<Integer>();
        Map<Integer,Database.SaveResult> res = new Map<Integer,Database.SaveResult>();
        for (Integer i = 0; i < all.size(); i++) {
            if (ops[i] == 'insert') { insertList.add(cands[i]); insertIdx.add(i); }
        }
        if (!insertList.isEmpty()) {
            Database.SaveResult[] r = Database.insert(insertList, false);
            for (Integer j = 0; j < r.size(); j++) res.put(insertIdx[j], r[j]);
        }
        for (Integer i = 0; i < all.size(); i++) {
            if (ops[i] == 'update') {
                Account u = cands[i].clone(false); u.Id = edits[ei++].Id;
                res.put(i, Database.update(u, false));
            } else if (ops[i] == 'upsert') {
                Account u = cands[i].clone(false); u.Ext__c = 'X' + String.valueOf(i);
                Database.UpsertResult ur = Database.upsert(u, Account.Ext__c, false);
                res.put(i, null);
                ei++;
                if (ur.isSuccess()) { outcomesU.put(i, 'S'); }
                else {
                    Database.Error[] errs = ur.getErrors();
                    String d = String.valueOf(errs[0].getStatusCode());
                    if (errs[0] instanceof Database.DuplicateError) { Datacloud.DuplicateResult dr = ((Database.DuplicateError) errs[0]).getDuplicateResult(); d = hits(dr, labels) + (dr.isAllowSave() ? '' : '!'); }
                    outcomesU.put(i, d);
                }
            }
        }
        List<String> outcomes = new List<String>();
        Integer savedCount = 0;
        for (Integer i = 0; i < all.size(); i++) {
            List<Datacloud.DuplicateResult> rules = found[i].getDuplicateResults();
            String f = '-';
            if (rules.size() == 1) { System.assertEquals(ruleName, rules[0].getDuplicateRule()); f = hits(rules[0], labels); }
            else f = 'rules' + String.valueOf(rules.size());
            String d = 'S';
            if (ops[i] == 'upsert') d = outcomesU.get(i);
            else {
                Database.SaveResult sr = res.get(i);
                if (sr.isSuccess()) d = 'S';
                else {
                    Database.Error[] errs = sr.getErrors();
                    String code = String.valueOf(errs[0].getStatusCode());
                    if (errs[0] instanceof Database.DuplicateError) { Datacloud.DuplicateResult dr = ((Database.DuplicateError) errs[0]).getDuplicateResult(); System.assertEquals(ruleName, dr.getDuplicateRule()); d = hits(dr, labels) + (dr.isAllowSave() ? '' : '!'); }
                    else d = 'E:' + code;
                }
            }
            if (d == 'S') savedCount++;
            outcomes.add(String.valueOf(i) + ':' + f + '/' + d);
        }
        System.assert(outcomes[outcomes.size() - 2].contains('='), 'positive control absent');
        System.assert(outcomes[outcomes.size() - 1].endsWith(':-/S'), 'negative control matched');
        return ruleName + '|saved=' + String.valueOf(savedCount) + '|' + String.join(outcomes, ',');
    }
    private static Map<Integer,String> outcomesU = new Map<Integer,String>();
    public static String m_gridZip0() {
        return run(new List<String>{
            'Cimrmv~https://hfxuwoc1.example~163 Ceftnos Avenue~_~40007~(409) 271-2896~Hopire~https://hfxuwoc1.example~163 Ceftnos Avenue~_~40007~(409) 271-2896~insert',
            'Wgjgru~https://cklafiz2.example~579 Xogazdd Drive~_~40014~(634) 691-5408~Kndovnc~https://cklafiz2.example~579 Xogazdd Drive~_~40014~634-691-5408~insert',
            'Cwferucu~https://rotozop3.example~319 Fofegom Street~_~40021~(256) 963-7759~Xerebuxu~https://rotozop3.example~319 Fofegom Street~_~40021~(256) 963-4080~insert',
            'Gucifare~https://sezujap4.example~400 Hemojov Avenue~_~40028~(944) 674-1021~Cocege~https://sezujap4.example~400 Hemojov Avenue~_~40028~(944) 311-1021~insert',
            'Jemuvo~https://cobrcel5.example~746 Kllezkk Street~_~40035~(414) 952-5868~Nunazd~https://cobrcel5.example~746 Kllezkk Street~_~40035~(751) 952-5868~insert',
            'Holeda~https://kohemis6.example~225 Goduruh Road~_~40042~(673) 266-7255~Badokct~https://kohemis6.example~225 Goduruh Road~_~40042~(310) 603-3576~insert',
            'Lecumi~https://taruges7.example~117 Viwuhej Lane~_~40049~(904) 210-7439~Jxveci~https://taruges7.example~117 Viwuhej Lane~_~40049~210-7439~insert',
            'Cebimal~https://joxtnot8.example~753 Riconzf Drive~_~40056~(832) 800-7117~Mstgvejo~https://joxtnot8.example~753 Riconzf Drive~_~40056~+1 8328007117~insert',
            'Prbetib~https://dowujak9.example~597 Lflipub Avenue~_~40063~(751) 600-9414~Xicuvu~https://dowujak9.example~597 Lflipub Avenue~_~40063~_~insert',
            'Cigohi~https://wapazul10.example~393 Huwexil Road~_~40070~_~Pinime~https://wapazul10.example~393 Huwexil Road~_~40070~(353) 697-2294~insert',
            'Zemkre~https://jgxanak11.example~384 Kogedok Avenue~_~40077~(442) 965-3149~Baxano~https://jgxanak11.example~387 Kogedok Avenue~_~40077~(442) 965-3149~insert',
            'Zedezoxu~https://gevomnx12.example~754 Mecakic Road~_~40084~(722) 618-5585~Jthimegi~https://gevomnx12.example~757 Mecakic Road~_~40084~722-618-5585~insert',
            'Pogutoc~https://tamoflt13.example~362 Cawamel Street~_~40091~(749) 520-1473~Kezlmi~https://tamoflt13.example~365 Cawamel Street~_~40091~(749) 520-6794~insert',
            'Mecuxak~https://pbxotsj14.example~421 Nigocil Road~_~40098~(585) 366-1268~Rgzonit~https://pbxotsj14.example~424 Nigocil Road~_~40098~(585) 703-1268~insert',
            'Cojilur~https://rurrjov15.example~176 Rxcikus Street~_~40105~(678) 473-8735~Hadiba~https://rurrjov15.example~179 Rxcikus Street~_~40105~(315) 473-8735~insert',
            'Cakexu~https://sipepam16.example~537 Likigob Drive~_~40112~(306) 433-9798~Murnlabi~https://sipepam16.example~530 Likigob Drive~_~40112~(643) 770-6119~insert',
            'Nrtede~https://cujongn17.example~177 Cefalzk Street~_~40119~(837) 291-9548~Cepewo~https://cujongn17.example~170 Cefalzk Street~_~40119~291-9548~insert',
            'Vzlbde~https://bipulux18.example~735 Kewomeh Drive~_~40126~(666) 823-1119~Golovxb~https://bipulux18.example~738 Kewomeh Drive~_~40126~+1 6668231119~insert',
            'Npcame~https://batuset19.example~371 Giducop Road~_~40133~(707) 975-4363~Wejewe~https://batuset19.example~374 Giducop Road~_~40133~_~insert',
            'Hurosp~https://mubicij20.example~241 Famitav Lane~_~40140~_~Hhdalobu~https://mubicij20.example~244 Famitav Lane~_~40140~(740) 745-1129~insert',
            'Bejrhlh~https://gutewvb21.example~756 Kuzoviw Road~_~40147~(493) 665-1868~Lagade~https://gutewvb21.example~374 Kuzoviw Road~_~40147~(493) 665-1868~insert',
            'Tsbazoru~https://bnjacis22.example~614 Pgvvpoz Lane~_~40154~(559) 549-1349~Lugana~https://bnjacis22.example~232 Pgvvpoz Lane~_~40154~559-549-1349~insert',
            'Vunetu~https://guzever23.example~303 Pokewnf Lane~_~40161~(270) 933-7821~Mxckgnmo~https://guzever23.example~721 Pokewnf Lane~_~40161~(270) 933-4142~insert'
        });
    }
    public static String m_gridZip1() {
        return run(new List<String>{
            'Nesswaw~https://muvabig24.example~871 Xerovdn Avenue~_~40168~(982) 870-2275~Vzkjrefe~https://muvabig24.example~489 Xerovdn Avenue~_~40168~(982) 507-2275~insert',
            'Rabafv~https://parelel25.example~664 Sezehol Drive~_~40175~(634) 918-5044~Cusatuta~https://parelel25.example~282 Sezehol Drive~_~40175~(271) 918-5044~insert',
            'Bimofug~https://kidunur26.example~498 Zcglker Avenue~_~40182~(929) 256-7331~Tivemov~https://kidunur26.example~116 Zcglker Avenue~_~40182~(566) 593-3652~insert',
            'Jsgodwf~https://bunucac27.example~846 Xarevir Drive~_~40189~(745) 454-5068~Famebopu~https://bunucac27.example~464 Xarevir Drive~_~40189~454-5068~insert',
            'Jezigzxo~https://fkwabeb28.example~318 Xazozlp Street~_~40196~(956) 693-9034~Volugf~https://fkwabeb28.example~736 Xazozlp Street~_~40196~+1 9566939034~insert',
            'Vocufoja~https://conutrm29.example~376 Gomijuw Drive~_~40203~(411) 485-6512~Zivalusu~https://conutrm29.example~794 Gomijuw Drive~_~40203~_~insert',
            'Fcfuca~https://sovibss30.example~292 Kidalbt Street~_~40210~_~Mikuvz~https://sovibss30.example~710 Kidalbt Street~_~40210~(313) 530-2329~insert',
            'Mefiro~https://voshtev31.example~877 Wfthwal Road~_~40217~(669) 419-3478~Nndehi~https://voshtev31.example~8770 Wfthwal Road~_~40217~(669) 419-3478~insert',
            'Guruzin~https://nivbtis32.example~711 Hefofib Lane~_~40224~(911) 566-8987~Gedixwh~https://nivbtis32.example~7110 Hefofib Lane~_~40224~911-566-8987~insert',
            'Zaxssz~https://mocuzij33.example~738 Vobvgcr Drive~_~40231~(614) 360-3173~Rehcfus~https://mocuzij33.example~7380 Vobvgcr Drive~_~40231~(614) 360-8494~insert',
            'Ketwmula~https://ffhurah34.example~462 Vawmlos Lane~_~40238~(440) 845-5255~Nnletal~https://ffhurah34.example~4620 Vawmlos Lane~_~40238~(440) 482-5255~insert',
            'Dijegx~https://pewapuw35.example~584 Fefagij Drive~_~40245~(981) 394-5643~Fawexa~https://pewapuw35.example~5840 Fefagij Drive~_~40245~(618) 394-5643~insert',
            'Furozkje~https://pidoluw36.example~635 Rnkejul Drive~_~40252~(371) 771-9578~Viperah~https://pidoluw36.example~6350 Rnkejul Drive~_~40252~(708) 408-5899~insert',
            'Mezisa~https://kureval37.example~682 Kgmopex Street~_~40259~(968) 702-2981~Cizowc~https://kureval37.example~6820 Kgmopex Street~_~40259~702-2981~insert',
            'Kgjiwij~https://pedezak38.example~818 Zibrtox Drive~_~40266~(474) 656-9686~Bevasoka~https://pedezak38.example~8180 Zibrtox Drive~_~40266~+1 4746569686~insert',
            'Refured~https://xixocuk39.example~405 Zenacog Drive~_~40273~(335) 989-2958~Nihafo~https://xixocuk39.example~4050 Zenacog Drive~_~40273~_~insert',
            'Komotiju~https://zintkus40.example~350 Hjvamic Street~_~40280~_~Tonocev~https://zintkus40.example~3500 Hjvamic Street~_~40280~(475) 254-7236~insert',
            'Dowininr~https://sowocun41.example~890 Gixicud Street~_~40287~(854) 756-8654~Zawifi~https://sowocun41.example~890 Gixicud St~_~40287~(854) 756-8654~insert',
            'Cucikvs~https://mevevat42.example~766 Wodnjwg Avenue~_~40294~(926) 518-1613~Bizldm~https://mevevat42.example~766 Wodnjwg Ave~_~40294~926-518-1613~insert',
            'Cekuwbp~https://zanoksl43.example~335 Liruguj Lane~_~40301~(844) 771-8893~Pilamaku~https://zanoksl43.example~335 Liruguj Ln~_~40301~(844) 771-5214~insert',
            'Cajatdke~https://xaloxam44.example~887 Wuzuldp Avenue~_~40308~(733) 761-2561~Mpgakiba~https://xaloxam44.example~887 Wuzuldp Ave~_~40308~(733) 398-2561~insert',
            'Juxiregt~https://davigah45.example~262 Fajsluc Drive~_~40315~(583) 635-4333~Fevrja~https://davigah45.example~262 Fajsluc Dr~_~40315~(220) 635-4333~insert',
            'Zizilat~https://pikimin46.example~888 Tsguruc Street~_~40322~(278) 462-3326~Rijaron~https://pikimin46.example~888 Tsguruc St~_~40322~(615) 799-8647~insert'
        });
    }
    public static String m_gridZip2() {
        return run(new List<String>{
            'Cubuduw~https://jefibap47.example~877 Jvvoloz Avenue~_~40329~(455) 330-9025~Fodeszs~https://jefibap47.example~877 Jvvoloz Ave~_~40329~330-9025~insert',
            'Habazuhi~https://hawwcoc48.example~545 Fpffdiz Avenue~_~40336~(744) 468-3766~Ppbuge~https://hawwcoc48.example~545 Fpffdiz Ave~_~40336~+1 7444683766~insert',
            'Bfkjfuc~https://pebukil49.example~549 Tecijin Lane~_~40343~(501) 213-4410~Tpmeloki~https://pebukil49.example~549 Tecijin Ln~_~40343~_~insert',
            'Nrjucoru~https://fajamoz50.example~132 Cewuxot Drive~_~40350~_~Ludurix~https://fajamoz50.example~132 Cewuxot Dr~_~40350~(476) 430-6786~insert',
            'Zaxpkawm~https://nazvcrk51.example~852 Cifumfb Drive~_~40357~(674) 927-7213~Wbrvje~https://nazvcrk51.example~852 Bumupec Drive~_~40357~(674) 927-7213~insert',
            'Xxdafomd~https://gaxizoc52.example~357 Vamufec Street~_~40364~(957) 716-9697~Mujovafo~https://gaxizoc52.example~357 Gxjopkg Street~_~40364~957-716-9697~insert',
            'Pkgejobu~https://remocav53.example~735 Lojezer Road~_~40371~(308) 404-9661~Ladakdt~https://remocav53.example~735 Pasufek Road~_~40371~(308) 404-5982~insert',
            'Botucoc~https://xdsezul54.example~878 Zidecap Avenue~_~40378~(711) 287-4264~Lomtlusr~https://xdsezul54.example~878 Thdolat Avenue~_~40378~(711) 624-4264~insert',
            'Zisofe~https://sefkwat55.example~588 Fowubel Road~_~40385~(589) 623-1339~Desacowo~https://sefkwat55.example~588 Kecofxx Road~_~40385~(226) 623-1339~insert',
            'Hekopu~https://xabekif56.example~163 Cobxjug Road~_~40392~(861) 823-8206~Jacadupo~https://xabekif56.example~163 Nexabor Road~_~40392~(498) 460-4527~insert',
            'Nehujan~https://turvboz57.example~538 Zhwimit Avenue~_~40399~(407) 206-9304~Jijovhhe~https://turvboz57.example~538 Vnhazig Avenue~_~40399~206-9304~insert',
            'Punami~https://lofmbib58.example~138 Lemazts Lane~_~40406~(334) 905-4095~Decehacu~https://lofmbib58.example~138 Cobison Lane~_~40406~+1 3349054095~insert',
            'Cxzujuri~https://lonkkim59.example~838 Bazovip Street~_~40413~(518) 565-9379~Flcbxon~https://lonkkim59.example~838 Bewotuw Street~_~40413~_~insert',
            'Zakuvud~https://hanzcuj60.example~313 Glfmwin Avenue~_~40420~_~Gajabum~https://hanzcuj60.example~313 Fnburlx Avenue~_~40420~(589) 763-2303~insert',
            'Cusiloc~https://jukmbug61.example~610 Bafaxir Street~_~40427~(247) 313-8535~Kapazut~https://jukmbug61.example~228 Lukstox Street~_~40427~(247) 313-8535~insert',
            'Taborali~https://sazczxt62.example~216 Zumicaf Street~_~40434~(816) 679-7288~Fazfba~https://sazczxt62.example~634 Juceklv Street~_~40434~816-679-7288~insert',
            'Fafobupi~https://wutanuf63.example~596 Mcregiv Lane~_~40441~(601) 469-2658~Sugftam~https://wutanuf63.example~214 Mawacac Lane~_~40441~(601) 469-7979~insert',
            'Bezehepo~https://feparux64.example~697 Pubebag Drive~_~40448~(287) 406-1650~Gbzijisr~https://feparux64.example~315 Cxgwtof Drive~_~40448~(287) 743-1650~insert',
            'Vumetuf~https://wonklnn65.example~105 Vcsilwz Avenue~_~40455~(978) 324-7526~Javodosd~https://wonklnn65.example~523 Fedapaf Avenue~_~40455~(615) 324-7526~insert',
            'Wubezod~https://durhhow66.example~236 Xiptmol Avenue~_~40462~(558) 746-4145~Hubohe~https://durhhow66.example~654 Ladonih Avenue~_~40462~(895) 383-9466~insert',
            'Rzratoce~https://dumawsg67.example~416 Vdzxdor Avenue~_~40469~(646) 323-8969~Xoturir~https://dumawsg67.example~834 Gbheder Avenue~_~40469~323-8969~insert',
            'Bikezb~https://ggnobes68.example~816 Jasupjz Street~_~40476~(549) 228-6652~Watakowo~https://ggnobes68.example~434 Tekukib Street~_~40476~+1 5492286652~insert',
            'Wekiwugo~https://ranehef69.example~203 Juvopeb Road~_~40483~(710) 235-9838~Ntjoscf~https://ranehef69.example~621 Wwtesin Road~_~40483~_~insert'
        });
    }
    public static String m_gridZip3() {
        return run(new List<String>{
            'Horifm~https://lifmxum70.example~495 Sspilos Lane~_~40490~_~Hiwujoxl~https://lifmxum70.example~113 Lalukod Lane~_~40490~(493) 956-3451~insert',
            'Vzsewevo~https://rizutin71.example~355 Zszohin Avenue~_~40497~(664) 369-2259~Nuruxo~https://rizutin71.example~_~_~40497~(664) 369-2259~insert',
            'Kobogop~https://bubaziz72.example~755 Teciguz Avenue~_~40504~(206) 564-2598~Nogewu~https://bubaziz72.example~_~_~40504~206-564-2598~insert',
            'Nisasaru~https://xuxokoj73.example~169 Tagupid Road~_~40511~(227) 269-3499~Pzrihi~https://xuxokoj73.example~_~_~40511~(227) 269-8820~insert',
            'Kexlsede~https://wmfmbiv74.example~428 Fuduwdj Avenue~_~40518~(374) 349-2385~Lobsze~https://wmfmbiv74.example~_~_~40518~(374) 686-2385~insert',
            'Tirugctu~https://mvvujep75.example~592 Necuzox Road~_~40525~(459) 823-2387~Bztinor~https://mvvujep75.example~_~_~40525~(796) 823-2387~insert',
            'Tibixhva~https://vuldgav76.example~511 Dipajuh Road~_~40532~(459) 826-5288~Bcwtzez~https://vuldgav76.example~_~_~40532~(796) 463-1609~insert',
            'Gifgle~https://fosidop77.example~797 Bucshix Lane~_~40539~(298) 253-4425~Jejanbj~https://fosidop77.example~_~_~40539~253-4425~insert',
            'Nibuhpc~https://fokuzaj78.example~626 Metdvom Road~_~40546~(279) 567-8319~Cuduklp~https://fokuzaj78.example~_~_~40546~+1 2795678319~insert',
            'Firfmo~https://titbtas79.example~225 Radejem Street~_~40553~(836) 478-6971~Fafwrm~https://titbtas79.example~_~_~40553~_~insert',
            'Flcopan~https://rugelug80.example~378 Nurgmos Drive~_~40560~_~Kcvefo~https://rugelug80.example~_~_~40560~(900) 522-1258~insert',
            'Tasixa~https://nevbwiw81.example~_~_~40567~(599) 886-4419~Reffckx~https://nevbwiw81.example~345 Lnntkrv Drive~_~40567~(599) 886-4419~insert',
            'Fakacat~https://jeropdx82.example~_~_~40574~(381) 464-4890~Vofane~https://jeropdx82.example~385 Ckfipij Lane~_~40574~381-464-4890~insert',
            'Winakit~https://pozgfed83.example~_~_~40581~(962) 776-2754~Capovice~https://pozgfed83.example~876 Gixefom Street~_~40581~(962) 776-8075~insert',
            'Bovitk~https://namocis84.example~_~_~40588~(406) 595-8826~Voturin~https://namocis84.example~637 Zenitab Street~_~40588~(406) 232-8826~insert',
            'Ncmihnno~https://conexjw85.example~_~_~40595~(223) 503-9195~Kvtofozb~https://conexjw85.example~544 Luxepog Drive~_~40595~(560) 503-9195~insert',
            'Jijobvl~https://sjmojuw86.example~_~_~40602~(761) 873-4561~Zopemel~https://sjmojuw86.example~650 Rivtjod Street~_~40602~(398) 510-9882~insert',
            'Lacogo~https://rufacem87.example~_~_~40609~(964) 210-2509~Dinotjz~https://rufacem87.example~894 Mevadub Avenue~_~40609~210-2509~insert',
            'Darexej~https://remejvx88.example~_~_~40616~(700) 905-7960~Dehubim~https://remejvx88.example~298 Welrtec Road~_~40616~+1 7009057960~insert',
            'Fagumpce~https://nohejxw89.example~_~_~40623~(406) 472-4012~Nigigono~https://nohejxw89.example~799 Hefupik Drive~_~40623~_~insert',
            'Bixoxa~https://kugepir90.example~_~_~40630~_~Zidufex~https://kugepir90.example~297 Herowok Drive~_~40630~(240) 644-1450~insert'
        });
    }
    public static String m_gridCity() {
        return run(new List<String>{
            'Jurvmoxe~https://nalegeg91.example~790 Wecumov Road~CtZetgse~40637~(571) 856-2640~Nosokex~https://nalegeg91.example~790 Wecumov Road~CtZetgse~80637~(571) 856-2640~insert',
            'Fizeke~https://gakojuk92.example~845 Hefigxt Road~CtKowehx~40644~(545) 403-1416~Ttwexu~https://gakojuk92.example~845 Hefigxt Road~CtKowehx~80644~(545) 403-6737~insert',
            'Hirkwe~https://zobasab93.example~648 Podimot Road~CtBoniph~40651~(898) 937-6661~Grhapvf~https://zobasab93.example~648 Podimot Road~CtBoniph~80651~(535) 574-2982~insert',
            'Zesojid~https://mldxlos94.example~597 Hamimoz Drive~CtWexura~40658~(847) 792-1538~Jbtavnln~https://mldxlos94.example~597 Hamimoz Drive~CtWexura~80658~_~insert',
            'Jucsnize~https://jotgjwg95.example~112 Jicodex Drive~CtXohixo~40665~(250) 258-5411~Frriti~https://jotgjwg95.example~530 Jicodex Drive~CtXohixo~80665~(250) 258-5411~insert',
            'Luboda~https://xagekun96.example~880 Loxlger Lane~CtSomepe~40672~(458) 382-8362~Zmcejohi~https://xagekun96.example~498 Loxlger Lane~CtSomepe~80672~(458) 382-4683~insert',
            'Filekiva~https://xevfjwl97.example~164 Xicbnal Drive~CtNexado~40679~(906) 774-3174~Vecicof~https://xevfjwl97.example~582 Xicbnal Drive~CtNexado~80679~(543) 411-8495~insert',
            'Ndroli~https://buvedut98.example~733 Zpmerez Drive~CtBizipi~40686~(884) 955-6056~Fupazc~https://buvedut98.example~351 Zpmerez Drive~CtBizipi~80686~_~insert',
            'Bkkara~https://tolosim99.example~264 Jopbvih Lane~CtDipoma~40693~(347) 384-5925~Keputih~https://tolosim99.example~682 Jexoxuz Lane~CtDipoma~80693~(347) 384-5925~insert',
            'Wohehur~https://zemezxw100.example~328 Nuwiklt Lane~CtZihzbx~40700~(246) 248-2411~Fopasow~https://zemezxw100.example~746 Wkkullj Lane~CtZihzbx~80700~(246) 248-7732~insert',
            'Jamucib~https://vzjumgw101.example~405 Nzhigus Lane~CtDusfhi~40707~(586) 431-6052~Lagepf~https://vzjumgw101.example~823 Jevonav Lane~CtDusfhi~80707~(223) 768-2373~insert',
            'Fejzba~https://horohex102.example~772 Gijexet Drive~CtNeradi~40714~(969) 915-1934~Satahot~https://horohex102.example~390 Cfxuhax Drive~CtNeradi~80714~_~insert',
            'Fjgusok~https://jotuwek103.example~795 Kzxazir Road~CtZeleki~40721~(455) 917-7044~Rvmfhu~https://jotuwek103.example~_~CtZeleki~80721~(455) 917-7044~insert',
            'Gunoruj~https://xoparoc104.example~483 Nmvrcut Drive~CtWazipl~40728~(894) 948-9385~Hewifad~https://xoparoc104.example~_~CtWazipl~80728~(894) 948-5706~insert',
            'Drcabi~https://fupokaf105.example~379 Kbkexrt Lane~CtKagzda~40735~(283) 525-4654~Vevirn~https://fupokaf105.example~_~CtKagzda~80735~(620) 862-9975~insert',
            'Txzawu~https://conupaw106.example~560 Zifsvex Lane~CtDumudo~40742~(770) 891-9267~Gupohfd~https://conupaw106.example~_~CtDumudo~80742~_~insert'
        });
    }
    public static String m_gateOff() {
        return run(new List<String>{
            'Hdtodu~https://cezpgol107.example~664 Nfdobuk Lane~CtMazeho~40749~(616) 594-6664~Kubxfrv~https://cezpgol107.example~664 Nfdobuk Lane~CtMazehox~80749~(616) 594-6664~insert',
            'Nosezh~https://xotvdiw108.example~603 Nejibed Avenue~CtSopumo~40756~(685) 570-6052~Zkbnxig~https://xotvdiw108.example~603 Nejibed Avenue~CtSopumox~80756~(685) 570-6052~insert',
            'Mutikp~https://tamodaf109.example~484 Wtrevop Avenue~CtNafepr~40763~(705) 647-5492~Dalkwu~https://tamodaf109.example~484 Wtrevop Avenue~CtNafeprx~80763~(705) 647-5492~insert',
            'Mxbucemg~https://zibelux110.example~116 Bilawak Lane~CtDumidl~40770~(955) 986-9132~Goduxapi~https://zibelux110.example~116 Bilawak Lane~CtDumidlx~80770~(955) 986-9132~insert',
            'Btponile~https://vplemah111.example~455 Lipthic Street~CtBuxizi~40777~(243) 625-5316~Zedjgewn~https://vplemah111.example~455 Lipthic Street~CtBuxizix~80777~(243) 625-5316~insert',
            'Filofon~https://lvdimux112.example~874 Gawifpd Street~CtJusaph~40784~(866) 250-3189~Rapigel~https://lvdimux112.example~874 Gawifpd Street~CtJusaphx~80784~(866) 250-3189~insert'
        });
    }
    public static String m_websiteVaried_W1a() {
        return run(new List<String>{
            'Bepuniji~https://xeznrnw113.example~553 Kefiteg Lane~_~40791~(914) 319-7151~Girekolz~http://xeznrnw113.example~556 Kefiteg Lane~_~40791~(551) 656-3472~insert',
            'Vemeci~https://jtladiv114.example~272 Rugifew Road~_~40798~(234) 388-5363~Veheto~http://jtladiv114.example~690 Rugifew Road~_~40798~(571) 725-1684~insert',
            'Kulpntje~https://kirhkos115.example~152 Kodaxos Lane~_~40805~(974) 953-2761~Hozalek~http://kirhkos115.example~570 Dadoxos Lane~_~40805~(974) 953-8082~insert',
            'Hosiwor~https://jocrrmt116.example~294 Nukvzuh Drive~_~40812~(508) 658-7254~Gepihat~http://jocrrmt116.example~712 Gjwuken Drive~_~40812~(508) 295-7254~insert',
            'Juzuburi~https://buvsknx117.example~594 Vikawit Avenue~_~40819~(326) 367-7240~Pklebucz~http://buvsknx117.example~212 Soxewih Avenue~_~40819~(663) 367-7240~insert',
            'Berajwt~https://baforax118.example~762 Wotelif Street~_~40826~(622) 835-1066~Sgbzcipo~http://baforax118.example~380 Tfreral Street~_~40826~835-1066~insert'
        });
    }
    public static String m_websiteVaried_W1b() {
        return run(new List<String>{
            'Wapuwej~https://gmdigtk119.example~380 Lonikoh Street~_~40833~(548) 698-3043~Znlobuvi~https://www.gmdigtk119.example~383 Lonikoh Street~_~40833~(885) 335-8364~insert',
            'Sebutedo~https://sucnfin120.example~415 Kibosej Street~_~40840~(334) 268-8420~Bsrijrc~https://www.sucnfin120.example~833 Kibosej Street~_~40840~(671) 605-4741~insert',
            'Mizefw~https://cimocet121.example~417 Hizefuf Avenue~_~40847~(650) 588-5306~Hihassku~https://www.cimocet121.example~835 Pacedvp Avenue~_~40847~(650) 588-1627~insert',
            'Fazede~https://hunmcit122.example~350 Bopovgj Drive~_~40854~(491) 714-6701~Gawulu~https://www.hunmcit122.example~768 Bicihit Drive~_~40854~(491) 351-6701~insert',
            'Nmjexo~https://lwzoxab123.example~669 Cuvppsk Street~_~40861~(925) 262-1091~Ksgpks~https://www.lwzoxab123.example~287 Cosuvzf Street~_~40861~(562) 262-1091~insert',
            'Ccsumbw~https://hctogih124.example~861 Zexftah Avenue~_~40868~(626) 949-7859~Rirunew~https://www.hctogih124.example~479 Nujokuv Avenue~_~40868~949-7859~insert'
        });
    }
    public static String m_websiteVaried_W2() {
        return run(new List<String>{
            'Tuputuk~https://gwdejiw125.example~770 Xkbobwm Avenue~_~40875~(501) 444-1899~Desogug~https://vwfltff125.example~773 Xkbobwm Avenue~_~40875~(838) 781-7220~insert',
            'Cucbre~https://zoxafat126.example~850 Mekataf Road~_~40882~(918) 438-8583~Piwhwjke~https://xokuluf126.example~468 Mekataf Road~_~40882~(555) 775-4904~insert',
            'Dallfow~https://bonvlag127.example~297 Joxitix Lane~_~40889~(979) 932-2826~Gubofif~https://gijugat127.example~715 Szzojej Lane~_~40889~(979) 932-8147~insert',
            'Xobepsfo~https://vikopap128.example~572 Radtjef Street~_~40896~(376) 431-4992~Somuteku~https://mubotut128.example~190 Bixirpd Street~_~40896~(376) 768-4992~insert',
            'Zaxahufa~https://tfjomkf129.example~807 Xrvakoc Street~_~40903~(385) 687-6583~Bajufe~https://xapidaw129.example~425 Xiwavas Street~_~40903~(722) 687-6583~insert',
            'Kapajima~https://pibidim130.example~226 Glbzbob Avenue~_~40910~(765) 796-1760~Hbkizep~https://fucikof130.example~644 Viwwsgm Avenue~_~40910~796-1760~insert'
        });
    }
    public static String m_namePath() {
        return run(new List<String>{
            'Cunosebs~https://vemusip131.example~582 Pmvivfx Street~CtRucoda~40917~(894) 924-4121~Cunosebs~_~200 Rujaxaw Street~CtRucoda~40917~(531) 561-9442~insert',
            'Rikitili~https://rorefeg132.example~898 Cagippt Street~CtMenuti~40924~(971) 788-6137~Rikitili~https://njxegvk132.example~516 Mecvruv Street~CtMenuti~40924~(608) 425-2458~insert'
        });
    }
    public static String m_phoneFormats() {
        return run(new List<String>{
            'Vifxckzf~https://pojuzkl133.example~141 Jamgvop Drive~_~40931~(315) 280-7174~Jodoxtto~https://pojuzkl133.example~141 Jamgvop Drive~_~40931~315.280.7174~insert',
            'Kagemu~https://koruwuk134.example~649 Boselih Avenue~_~40938~(720) 315-5205~Werilvdu~https://koruwuk134.example~649 Boselih Avenue~_~40938~7203155205~insert',
            'Wjkukevf~https://kfhbjag135.example~205 Zoredzb Drive~_~40945~(719) 984-5677~Demexohi~https://kfhbjag135.example~205 Zoredzb Drive~_~40945~(719) 984-5677 x12~insert',
            'Nusehes~https://vucitip136.example~651 Fehasux Drive~_~40952~(777) 479-8073~Rorinikg~https://vucitip136.example~269 Swkihax Drive~_~40952~777.479.8073~insert',
            'Firuka~https://rehenep137.example~178 Venagih Lane~_~40959~(832) 509-5614~Fadecu~https://rehenep137.example~596 Hiwavam Lane~_~40959~8325095614~insert',
            'Wrsbne~https://meciguf138.example~530 Bupidux Street~_~40966~(851) 900-6789~Bnpimi~https://meciguf138.example~148 Kohtwic Street~_~40966~(851) 900-6789 x12~insert'
        });
    }
    public static String m_edit_update() {
        return run(new List<String>{
            'Zexldt~https://laxurdw139.example~354 Sosirap Avenue~_~40973~(355) 232-6012~Xemihnte~https://laxurdw139.example~357 Sosirap Avenue~_~40973~(692) 569-2333~update',
            'Zumema~https://vwjofav140.example~581 Kuvvhnt Lane~_~40980~(716) 623-8468~Bvzove~https://vwjofav140.example~199 Hvxkcot Lane~_~40980~(716) 623-4789~update'
        });
    }
    public static String m_edit_upsert() {
        return run(new List<String>{
            'Zegumac~https://kolobtf141.example~504 Nuxpvsw Street~_~40987~(572) 964-6317~Xokfxen~https://kolobtf141.example~507 Nuxpvsw Street~_~40987~(209) 601-2638~upsert',
            'Rokupuno~https://ndvotic142.example~709 Jcbilux Street~_~40994~(499) 237-2448~Japcbx~https://ndvotic142.example~327 Covoxaz Street~_~40994~(499) 237-7769~upsert'
        });
    }
}
