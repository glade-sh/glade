package dml

import "strings"

// Dictionary tables proven by the native rows in
// internal/apextest/testdata/conformance/duplicate_fuzzy_matching/org.tsv.
// Salesforce does not publish its Name Variant and Acronym dictionaries;
// every entry here is backed by a captured pair, and
// TestFuzzyNameTablesMatchNativeRows keeps the first-name tables equal to
// what the rows show.

// nameVariantTable lists, per candidate first name, the existing names the
// native rows find at confidence 100 where Jaro-Winkler does not explain the
// score (Name Variant entries). The dictionary is directional.
const nameVariantTable = `
aaron aron
abby abigail
abe abraham abram ibrahim
abigail abby gail
abraham abe abram ibrahim
abram abe abraham ibrahim
addie adelaide
adelaide addie
agnes agnieszka
agnieszka agnes
aidan aiden
aiden aidan
aiko aikoh
aikoh aiko
akio akioh
akioh akio
al albert alberto alicia elbert
alain alan allen
alan alain allan allen
albert al alberto elbert
alberto al albert elbert
alec alejandro alessandro alex alexander
alejandra alex alexa alexandra alexandria
alejandro alec alessandro alex alexander alexandre
aleksander alex alexander alexandr
alessandro alec alejandro alex alexander alexandre
alessia alexa
alex alec alejandra alejandro aleksander alessandro alexa alexander alexandr alexandra alexandre alexandria
alexa alejandra alessia alex alexandra alexandria alexis
alexander alec alejandro aleksander alessandro alex alexandr alexandre
alexandr aleksander alex alexander
alexandra alejandra alex alexa
alexandre alejandro alessandro alex alexander
alexandria alejandra alex alexa
alexey alexis
alexis alexa alexey
alfonso alonzo
alfred alfredo
alfredo alfred
alice alicia alisha alison allie allison ally allyson alyssa
alicia alice alisha alison allie allison ally allyson alyssa
alisha alice alicia alison allie allison ally allyson alyssa
alison alice alicia alisha allie allison ally allyson alyssa
allan alan allen
allen alain alan allan
allie alice alicia alisha alison allison ally allyson alyssa
allison alice alicia alisha alison allie ally allyson alyssa
ally alice alicia alisha alison allie allison allyson alyssa
allyson alice alicia alisha alison allie allison ally alyssa
alonzo alfonso
alysha alyssa
alyssa alice alicia alisha alison allie allison ally allyson alysha
amalia amelia amelie
amanda mandy
amelia amalia amelie emilee emilia emilie emily emmy
amelie amalia amelia
amie amy
amira amirah
amirah amira
amit amith
amy amie
ana anja ann anna annabel annabelle annabeth anne anneke annette annie annika
anat anath
anath anat
anders anderson
anderson anders
andre andrea andrew
andrea andrei andres andrew andrzej andy
andrei andrea andres andrew andrzej andy
andres andrea andrei andrew andrzej andrés andy
andrew andrea andrei andres andrzej andy
andrzej andrea andrei andres andrew andy
andrés andres
andy andrea andrei andres andrew andrzej
angel angela angelina angelo angie
angela angel angelina angie
angelina angel angela angie
angelo angel
angie angel angela angelina
anja ana ann anna annabel annabelle annabeth anne anneke annelies annette annie annika
ann ana anja anna annabel annabelle annabeth anne anneke annelies annette annie annika
anna ana anja ann annabel annabelle annabeth anne anneke annelies annette annie annika
annabel ana anja ann anna annabelle annabeth anne anneke annette annie annika
annabelle ana anja ann anna annabel annabeth anne anneke annelies annette annie annika
annabeth ana anja ann anna annabel annabelle anne anneke annette annie annika
anne ana anja ann anna annabel annabelle annabeth anneke annemarie annette annie annika
anneke ana anja ann anna annabel annabelle annabeth anne annelies annette annie annika
annelies anja ann anna annabelle anneke annette annie annika
annemarie anne
annette ana anja ann anna annabel annabelle annabeth anne anneke annelies annie annika
annie ana anja ann anna annabel annabelle annabeth anne anneke annelies annette annika
annika ana anja ann anna annabel annabelle annabeth anne anneke annelies annette annie
anthony antoine anton antonio antony tony
antoine anthony anton antonio tony
antoinette antonia
anton anthony antoine antonio tony
antonia antoinette
antonio anthony antoine anton tony
antony anthony
archibald archie
archie archibald
ari arih
ariana arianna
arianna ariana
arih ari
armand armando
armando armand
arnie arnold
arnold arnie
aron aaron
art arthur
arthur art arturo
arturo arthur
ashleigh ashley
ashley ashleigh ashlyn
ashlyn ashley
aubree aubrey
aubrey aubree
august augustus gus
augusta gus
augustus august gus
ava eva eve evelyn
bao baoh
baoh bao
barb barbara barbie
barbara barb barbie
barbie barb barbara
barney bernard bernie
beata beatrice
beatrice beata beatrix beatriz
beatrix beatrice beatriz
beatriz beatrice beatrix
becky reba rebecca rebekah
ben benedict benito benjamin benji benjy bennie benny
benedict ben benito benny
benito ben benedict benny
benjamin ben benji benjy bennie benny
benji ben benjamin bennie benny
benjy ben benjamin
bennie ben benjamin benji benny
benny ben benedict benito benjamin benji bennie
bernard barney bernie
bernie barney bernard
bessie beth betsy betty
beth bessie betsy betty elisa elisabeth elise eliza elizabeth elsa libby liz
betsy bessie beth betty elisabeth eliza elizabeth elsa libby liz
betty bessie beth betsy elisa elisabeth elise eliza elizabeth elsa libby liz
bill billy will willem william willie willy
billy bill will willem william willie willy
blanca blanche
blanche blanca
bo boh
bob bobbie bobby rob robbie robert roberto robin
bobbie bob bobby rob robbie robert robin robyn
bobby bob bobbie rob robbie robert roberto robin robyn
boh bo
brad bradford bradley
braden brayden
bradford brad
bradley brad
brandi brandy
brandon brendan brennan
brandy brandi
brayden braden
brendan brandon brennan
brennan brandon brendan
brian bryan bryant
bridget brigitte
brigitte bridget
britney brittany
brittany britney brittney
brittney brittany
brooke brooklyn
brooklyn brooke
bryan brian bryant
bryant brian bryan
bryce bryson
bryson bryce
bud buddy
buddy bud
caio caioh
caioh caio
caitlin catarina kaitlyn katalin katarina katarzyna katelyn katerina katie katrin katrina katy katya
cal calvin
caleb kaleb
calista callie
callie calista
calvin cal
cam cameron
cameron cam
camila camille
camille camila camilo
camilo camille
candace candice
candice candace
carl carlos carol karl
carla carly carol carolina caroline carolyn carrie karla
carlos carl carol karl
carly carla carol carolina caroline carolyn carrie karla
carmela carmen
carmen carmela
carol carl carla carlos carly carole carolina caroline carolyn carrie karl
carole carol caroline
carolina carla carly carol caroline carolyn carrie karolina
caroline carla carly carol carole carolina carolyn carrie
carolyn carla carly carol carolina caroline carrie
carrie carla carly carol carolina caroline carolyn karla
cassandra kassandra
catalina catarina
catarina caitlin catalina kaitlyn katalin katarina katarzyna katelyn katerina katie katrin katrina katy katya
catherine cathy kate katherine kathryn kathy kay kitty
cathleen cathy kathleen kathy
cathy catherine cathleen kate katherine kathleen kathryn kathy kay kitty
cecilia sissy
charlene charlie charlotte lottie sharlene
charles charlie chuck
charlie charlene charles charlotte chuck lottie sharlene
charlotte charlene charlie lottie
chas chaz
chaz chas
cherie cheryl sheri sherri sherry
cheryl cherie sheri sherri sherry
chidi chihdi
chihdi chidi
chiyo chiyoh
chiyoh chiyo
chris chrissie chrissy christa christian christiane christina christine christopher christy cristian cristina kirsten kris krista kristen kristian kristin kristina kristoffer kristopher kristy tina
chrissie chris chrissy
chrissy chris chrissie christa christiane christina christine christy cristina kirsten kris krista kristen kristin kristina kristy tina
christa chris chrissy christiane christine christy kris krista kristy
christian chris cristian kris kristian
christiane chris chrissy christa christy kris krista kristy
christina chris chrissy christine christy cristina kirsten kris kristen kristin kristina tina
christine chris chrissy christa christina christy cristina kirsten kris krista kristen kristin kristina kristy tina
christopher chris christy kris kristoffer kristopher
christy chris chrissy christa christiane christina christine christopher cristina kirsten kris krista kristen kristin kristina kristy tina
chuck charles charlie
claire clara
clara claire
claude claudia gladys
claudia claude gladys
clay clayton
clayton clay
cliff clifford
clifford cliff
clint clinton
clinton clint
colin collin colm klaus
collin colin
colm colin
colten colton
colton colten
conner connor
connie constance
connor conner
constance connie
constantine konstantin
cora corinne
corey cory
corinne cora
cornelis cornelius
cornelius cornelis
cory corey
courtney curt curtis
cristian chris christian kris kristian
cristina chris chrissy christina christine christy kirsten kris kristen kristin kristina tina
cristobal christopher
curt courtney curtis kurt kurtis
curtis courtney curt kurtis
daan dan
dahlia dalia
dalia dahlia
damian damien
damien damian
dan daan dani daniel danny
dani dan daniel daniela danielle danny danuta
daniel dan dani danny
daniela dani danielle danuta
danielle dani daniela danuta
danny dan dani daniel
danuta dani daniela danielle
dara dahra darah
darian dario darius dariusz
dario darian darius dariusz
darius darian dario dariusz
dariusz darian dario darius
darrell darryl daryl
darren darryl daryl
darryl darrell darren daryl
daryl darrell darren darryl
dave davey david davide davis
davey dave david davide davis
david dave davey davide davis
davide dave davey david davis
davis dave davey david davide
deanna diana diane dianne
deb debbie debby deborah debra
debbie deb debby deborah debra
debby deb debbie deborah debra
debora deborah
deborah deb debbie debby debora debra
debra deb debbie debby deborah
debrah deborah
delores dolly
denis dennis
dennis denis denny
denny dennis
derek derrick dirk
derrick derek dirk
devin devon
devon devin
dewey dewi
dewi dewey
diana deanna diane dianne
diane deanna diana dianne
dianne deanna diana diane
dick rich richard richie rick
dimitri dmitri dmitry
dina dinah
dinah dina
dirk derek derrick
dmitri dimitri dmitry
dmitry dimitri dmitri
dolly delores dora dorota dorothea dorothy
dom dominic dominick dominik dominique
dominic dom dominick dominik dominique
dominick dom dominic dominik dominique
dominik dom dominic dominick dominique
dominique dom dominic dominick dominik
don donal donald donnie
donal don donald donnie
donald don donal donnie
donnie don donal donald
dora dolly dorota dorothea dorothy
dorota dolly dora dorothea dorothy
dorothea dolly dora dorota dorothy
dorothy dolly dora dorota dorothea dot
dot dorothy
duane dwain dwayne
dwain duane dwayne
dwayne duane dwain
ed eddie eddy edmund eduardo edward ned
eddie ed eddy edmund eduardo edward ned
eddy ed eddie edmund eduardo edward ned
edgar edgardo
edgardo edgar
edison eduardo edward
edmund ed eddie eddy ned
eduardo ed eddie eddy edison edward ned
edward ed eddie eddy edison eduardo ned
elaine elena eleni ella ellen ilona
elbert al albert alberto
eleanor ella ellen ellie nell
elena elaine eleanor eleni ella ellen ilona
eleni elaine elena ella ellen
eliana eleanor
elias elijah eliot elliot elliott ellis
elijah elias eliot elliot elliott ellis
eliot elias elijah elliot elliott ellis
elisa beth betty elisabeth elise eliza elizabeth elsa elsie liz
elisabeth beth betsy betty elisa elise eliza elizabeth elsa elsie libby liz
elise beth betty elisa elisabeth eliza elizabeth elsa elsie liz
eliza beth betsy betty elisa elisabeth elise elizabeth elsa elsie libby liz
elizabeth beth betsy betty elisa elisabeth elise eliza elsa elsie libby liz
ella elaine eleanor elena eleni ellen ellie
ellen elaine eleanor elena eleni ella ellie ilona nell
ellie eleanor ella ellen nell
elliot elias elijah eliot elliott ellis
elliott elias elijah eliot elliot ellis
ellis elias elijah eliot elliot elliott
elsa beth betsy betty elisa elisabeth elise eliza elizabeth elsie libby liz
elsie elisa elisabeth elise eliza elizabeth elsa
emanuel emanuele emmanuel
emanuele emanuel emmanuel
emerson emery emory
emery emerson emory
emil emiliano emilio
emilee amelia emilia emilie emily emmy
emilia amelia emilee emilie emily emmy
emiliano emil emilio
emilie amelia emilee emilia emily emmy
emilio emil emiliano
emily amelia emilee emilia emilie emmy
emmanuel emanuel emanuele
emmy amelia emilee emilia emilie emily
emory emerson emery
enrique enzo
enzo enrique enzoh
enzoh enzo
eric erich erick erik
erica erika
erich eric erik
erick eric erik
erik eric erich erick
erika erica
ernest ernesto ernie ernst
ernesto ernest ernie ernst
ernie ernest ernesto ernst
ernst ernest ernesto ernie
ervin irvin
estela estelle
estelle estela
eva ava eve
eve ava eva
evelyn ava
fabian fabio
fabio fabian
fanny fran frances francesca francis frankie frannie
fatima fatma
fatma fatima
faye fayeh
fayeh faye
ferdinand fernando
fern fernanda fernando
fernanda fern
fernando ferdinand fern
flor flora
flora flor
florian florin
florin florian
fran fanny frances francesca francine francis frankie frannie
frances fanny fran francesca francine francis frankie frannie
francesca fanny fran frances francine francis frankie frannie
francesco francis francisco frank frankie
francine fran frances francesca francis
francis fanny fran frances francesca francesco francine francisco frank frankie frannie
francisco fran francesco francis frank frankie
frank francesco francis francisco frankie
frankie fanny fran frances francesca francesco francis francisco frank frannie
frannie fanny fran frances francesca francis frankie
fred freddie freddy frederick fritz
freddie fred freddy frederica frederick fritz
freddy fred freddie frederick fritz
frederica freddie
frederick fred freddie freddy fritz
freja freya
freya freja
fritz fred freddie freddy frederick
gabby gabe gabriel gabriela gabriella gabrielle
gabe gabby gabriel
gabriel gabby gabe
gabriela gabby gabriella gabrielle
gabriella gabby gabriela gabrielle
gabrielle gabby gabriela gabriella
gail abigail
garrett gerrit
garry gary
gary garry
geoff geoffrey jeff jeffrey
geoffrey geoff jeff jeffery jeffrey jeffry
george jerzy jorge jurgen
georgia georgiana georgina giorgia jorge
georgiana georgia georgina giorgia
georgina georgia georgiana giorgia jorge
gerald gerry
geraldine gerry
gerard gerardo gerhard gerrit
gerardo gerard gerhard gerrit
gerhard gerard gerardo gerrit
gerrit garrett gerard gerardo gerhard
gerry gerald geraldine
gil gilbert gilles
gilbert gil
gilles gil
ginger ginny
ginny ginger
giorgia georgia georgiana georgina
gisela giselle
giselle gisela
giulia julia juliana julianna julianne julie juliet juliette
giuseppe joe josef joseph
gladys claude claudia
glen glenn
glenn glen
grace gracie
gracie grace
greg gregory
gregory greg
greta maggie margaret margarita margaux marge margie margo margot marguerite marjorie meg peggy rita
guillaume guillermo
guillermo guillaume
gunnar gunter
gunter gunnar
gus august augustus
gustav gustavo
gustavo gustav
gwen gwendolyn
gwendolyn gwen
hailey haley
hakeem hakim
hakim hakeem
haley hailey
hana hanna
hanah hana
hank hans hendrik henri henrik henry
hanna hannah
hannah hanna
hans hank
hari harih
harih hari
harriet hattie
harris harrison harry
harrison harris harry
harry harris harrison
haruto harutoh
harutoh haruto
hattie harriet
hedda hedwig
hedwig hedda
hehmi hemi
heinz hendrik henk henri henrik henry
helmut helmuth
helmuth helmut
hemi hehmi
hendrik hank heinz henk henri henrik henry
henk heinz hendrik henri henrik henry
henri hank heinz hendrik henk henrih henrik henry
henrih henri
henrik hank heinz hendrik henk henri henry
henry hank heinz hendrik henk henri henrik
herb herbert
herbert herb
ibrahim abe abraham abram
ilona elaine elena ellen
ines inez
inez ines
ioana ioanna
ioanna ioana
ira irah irene irina
irene ira irina
irina ira irene
irvin ervin irving
irving irvin
isabel isabela isabella isabelle izzy
isabela isabel isabella isabelle izzy
isabella isabel isabela isabelle izzy
isabelle isabel isabela isabella izzy
isadora izzy
isaiah isaias isiah
isaias isaiah isiah
isiah isaiah isaias
izzy isabel isabela isabella isabelle isadora
j j_r j.
j_r j j.
j. j j_r
jack jackie jackson jake jaxon
jackie jack jackson jaclyn jacob jacqueline jacques jake jakub jaxon
jackson jack jackie jake jaxon
jaclyn jackie jacqueline
jacob jackie jacques jakub
jacqueline jackie jaclyn
jacques jackie jacob jakub
jada jade jaden jaiden jayden
jade jada jaden jaiden jayden
jaden jada jade jaiden jayden
jaiden jada jade jaden jayden
jaime james jamie
jake jack jackie jackson jaxon
jakub jackie jacob jacques
james jaime jamie jamison jim jimmie jimmy
jamie jaime james jamison jim jimmie jimmy
jamison james jamie jim jimmie jimmy
jan jana janelle janice janie janis jean jeanette jeannette jenna jennie jenny jens joan joana joann joanna joanne john johnny jon juan juana juanita
jana jan janelle janice janie janis jenna jenny joan joanne
jane janet jeanne
janelle jan jana janice janie janis jean jeanette jenny joan joana joann joanna joanne
janet jane jeanne
janice jan jana janelle janie janis jean jeanette jenna jenny joan joana joann joanna joanne
janie jan jana janelle janice janis jenna jenny joan
janis jan jana janelle janice janie jenna jenny joan
janne jayne john
jared jerrod
jarrett jerry
jasmin jasmine jazmine
jasmine jasmin jazmine
jason jayson
jaxon jack jackie jackson jake
jayden jada jade jaden jaiden
jayla jaylee
jaylee jayla
jayne janne
jayson jason
jazmine jasmin jasmine
jean jan janelle janice jeanette jeannette jenna jennie jenny jens joan joana joann joanna joanne john johnnie johnny jon jonas juan juana juanita
jeanette jan janelle janice jean jeannette jenna jennie jenny joan joana joann joanna joanne
jeanne jane janet
jeannette jan jean jeanette jenna jennie jenny
jed jedidiah
jedidiah jed
jeff geoff geoffrey jefferson jeffery jeffrey jeffry
jefferson jeff jeffery jeffrey jeffry
jeffery geoffrey jeff jefferson jeffrey jeffry
jeffrey geoff geoffrey jeff jefferson jeffery jeffry
jeffry geoffrey jeff jefferson jeffery jeffrey
jen jenifer jenna jennie jennifer jenny
jenifer jen jenna jennifer jenny
jenna jan jana janice janie janis jean jeanette jeannette jen jenifer jennie jennifer jenny joan joanne
jennie jan jean jeanette jeannette jen jenna jennifer jenny joan
jennifer jen jenifer jenna jennie jenny
jenny jan jana janelle janice janie janis jean jeanette jeannette jen jenifer jenna jennie jennifer joan joana joann joanna joanne
jens jan jean
jerald jerry
jeremiah jeremy jerry
jeremy jeremiah jerry
jeroen jerome
jerome jeroen
jerrod jared
jerry jarrett jerald jeremiah jeremy
jerzy george jorge jurgen
jess jesse jessica jessie
jesse jess jessie
jessica jess jessie
jessie jess jesse jessica
jiho jihoh
jihoh jiho
jill jillian
jillian jill
jim james jamie jamison jimmie jimmy
jimbo jimboh
jimboh jimbo
jimmie james jamie jamison jim jimmy
jimmy james jamie jamison jim jimmie
jin jing
jing jin
jiri jirih
jirih jiri
jo joe josef josefina joseph josephine josie
joan jan jana janelle janice janie janis jean jeanette jenna jennie jenny joana joann joanna joanne john johnny jon jonas juan juana juanita
joana jan janelle janice jean jeanette jenny joan joann joanna joanne juana juanita
joann jan janelle janice jean jeanette jenny joan joana joanna joanne
joanna jan janelle janice jean jeanette jenny joan joana joann joanne
joanne jan jana janelle janice jean jeanette jenna jenny joan joana joann joanna
joaquin joaquim
jocelyn joselyn joss
jodi jodie jody judit judith judy
jodie jodi jody judit judith judy
jody jodi jodie jude judit judith judy
joe giuseppe jo josef joseph
john jan janne jean joan johnnie johnny jon jonas juan
johnnie jean john johnny jon jonas juan
johnny jan jean joan john johnnie jon jonas juan
jon jan jean joan john johnnie johnny jonas jonathan jonathon juan
jonah jonas
jonas jean joan john johnnie johnny jon jonah juan
jonathan jon jonathon
jonathon jon jonathan
jordan jordyn
jordyn jordan
jorge george jerzy jurgen
josef giuseppe jo joe joseph
josefina jo josephine josie
joselyn jocelyn joss
joseph giuseppe jo joe josef
josephine jo josefina josie
josh joshua
joshua josh
josie jo josefina josephine
joss jocelyn joselyn
juan jan jean joan john johnnie johnny jon jonas
juana jan jean joan joana juanita
juanita jan jean joan joana juana
jude jody
judit jodi jodie jody judith judy
judith jodi jodie jody judit judy
judy jodi jodie jody judit judith
julia giulia juliana julianna julianne julie juliet juliette
julian julio julius
juliana giulia julia julianna julianne julie juliet juliette
julianna giulia julia juliana julianne julie juliet juliette
julianne giulia julia juliana julianna julie juliet juliette
julie giulia julia juliana julianna julianne juliet juliette
juliet giulia julia juliana julianna julianne julie juliette
juliette giulia julia juliana julianna julianne julie juliet
julio julian julius
julius julian julio
jurgen george jerzy jorge
justin justus
justus justin
kai kaia
kaia kai
kaitlyn caitlin catarina katalin katarina katarzyna katelyn katerina katie katrin katrina katy katya
kaito kaitoh
kaitoh kaito
kaleb caleb
kaori kaorih
kaorih kaori
kara karen karin karina karyn
karen kara karin karina karyn
karin kara karen karina karyn
karina kara karen karin karyn
karl carl carlos carol
karla carla carly carrie
karolina carolina
karyn kara karen karin karina
kasey kassandra
kassandra cassandra kasey
kat katalin katarina katarzyna katie
katalin caitlin catarina kaitlyn kat katarina katarzyna katelyn katerina katie katrin katrina katy katya
katarina caitlin catarina kaitlyn kat katalin katarzyna katelyn katerina katie katrin katrina katy katya
katarzyna caitlin catarina kaitlyn kat katalin katarina katelyn katerina katie katrin katrina katy katya
kate catherine cathy katherine kathryn kathy kay kitty
katee katie
katelyn caitlin catarina kaitlyn katalin katarina katarzyna katerina katie katrin katrina katy katya
katerina caitlin catarina kaitlyn katalin katarina katarzyna katelyn katie katrin katrina katy katya
katherine catherine cathy kate kathleen kathryn kathy kay kitty
kathleen cathleen cathy katherine kathryn kathy kay
kathryn catherine cathy kate katherine kathleen kathy kay kitty
kathy catherine cathleen cathy kate katherine kathleen kathryn kay kitty
katie caitlin catarina kaitlyn kat katalin katarina katarzyna katee katelyn katerina katrin katrina katy katya
katrin caitlin catarina kaitlyn katalin katarina katarzyna katelyn katerina katie katrina katy katya
katrina caitlin catarina kaitlyn katalin katarina katarzyna katelyn katerina katie katrin katy katya
katy caitlin catarina kaitlyn katalin katarina katarzyna katelyn katerina katie katrin katrina katya
katya caitlin catarina kaitlyn katalin katarina katarzyna katelyn katerina katie katrin katrina katy
kay catherine cathy kate katherine kathleen kathryn kathy kitty
kayla kaylee kayleigh
kaylee kayla kayleigh
kayleigh kayla kaylee
kazuo kazuoh
kazuoh kazuo
keiko keikoh
keikoh keiko
keira kiara
ken kenneth kenny
kenneth ken kenny
kenny ken kenneth
khaled khalid
khalid khaled
kiara keira
kim kimberly
kimberly kim
kira kyra
kirsten chris chrissy christina christine christy cristina kris kristen kristin kristina tina
kit kitty
kitty catherine cathy kate katherine kathryn kathy kay kit
klaus colin
kobe kohbe
kohbe kobe
kohji koji
kohri kori
koji kohji
konstantin constantine
kori kohri
kris chris chrissy christa christian christiane christina christine christopher christy cristian cristina kirsten krista kristen kristian kristin kristina kristoffer kristopher kristy tina
krista chris chrissy christa christiane christine christy kris kristen kristin kristina kristy
kristen chris chrissy christina christine christy cristina kirsten kris krista kristian kristin kristina kristy tina
kristian chris christian cristian kris kristen
kristin chris chrissy christina christine christy cristina kirsten kris krista kristen kristina kristy tina
kristina chris chrissy christina christine christy cristina kirsten kris krista kristen kristin kristy tina
kristoffer chris christopher kris kristopher krzysztof
kristopher chris christopher kris kristoffer krzysztof
kristy chris chrissy christa christiane christine christy kris krista kristen kristin kristina
krzysztof kristoffer kristopher
kurt curt
kurtis curt curtis
kyla kylie
kylie kyla
kyra kira
laila layla leila lela leyla
landon lanny
lanny landon
lara laura laurie lori
larry lars laurence laurent laurie lawrence lorenzo
lars larry laurence laurent laurie lawrence lorenzo
laura lara laurie lori
lauren laurence laurie lauryn
laurence larry lars lauren laurent laurie lauryn lawrence lorenzo
laurent larry lars laurence laurie lawrence lorenzo
laurie lara larry lars laura lauren laurence laurent lauryn lawrence lorena lorene lorenzo lori
lauryn lauren laurence laurie
lawrence larry lars laurence laurent laurie lorenzo
layla laila leila lela leyla
lea leah
leah lea
leila laila layla leilah lela leyla
leilah leila lela
lela laila layla leila leilah leyla
len lenny leonard leonardo
lena lenka
lenka lena
lenny len leonard leonardo
lenora leonor leonora
leo leon
leon leo
leonard len lenny leonardo
leonardo len lenny leonard
leonor lenora leonora
leonora lenora leonor
lesley leslie lessie
leslie lesley lessie
lessie lesley leslie
lewis lou louie louis luis
leyla laila layla leila lela
libby beth betsy betty elisabeth eliza elizabeth elsa liz
lilian liliana lillian
liliana lilian lillian
lillian lilian liliana
lillie lilly lily
lilly lillie lily
lily lillie lilly
lindsay lindsey
lindsey lindsay
lindy lynda lynn lynne
lisa lisette lizbeth lizzie
lisette lisa lizbeth lizzie
liz beth betsy betty elisa elisabeth elise eliza elizabeth elsa libby
lizbeth lisa lisette lizzie
lizzie lisa lisette lizbeth
lonnie lonny
lonny lonnie
lorena laurie
lorene laurie lori
lorenzo larry lars laurence laurent laurie lawrence
lori lara laura laurie lorene lorraine
lorraine lori
lottie charlene charlie charlotte
lou lewis louie louis louisa louise luis
louie lewis lou louis luis
louis lewis lou louie luis
louisa lou louise
louise lou louisa
luc luca lucas luka lukas luke
luca luc lucas luka lukas luke
lucas luc luca luka lukas luke
lucia luciana lucille lucy
lucian luciano lucio
luciana lucia lucille lucy
luciano lucian lucio
lucille lucia luciana lucy
lucio lucian luciano
lucy lucia luciana lucille
luis lewis lou louie louis
luka luc luca lucas lukas luke
lukas luc luca lucas luka luke
luke luc luca lucas luka lukas
lupe lupita
lupita lupe
lynda lindy lynn lynne
lynn lindy lynda lynne
lynne lindy lynda lynn
maddie madeleine madeline madelyn
madeleine maddie madeline madelyn
madeline maddie madeleine madelyn
madelyn maddie madeleine madeline
madison mattie maud
maggie greta mairead margaret margarita margaux marge margie margo margot marguerite marjorie meg peggy rita
mairead maggie
maisie maisieh
maisieh maisie
makayla michaela mickey mikaela mikayla
mamie mimi
mandy amanda
marc marcelo marco marcus marek mark marko markus
marcel marcelo marcus
marcella marcia marsha
marcelo marc marcel marco marcus marek mark marko markus
marcia marcella marsha
marco marc marcelo marcus marek mark marko markus
marcus marc marcel marcelo marco marek mark marko markus
marek marc marcelo marco marcus mark marko markus
margaret greta maggie margarida margarita margaux marge margie margo margot marguerite marjorie meg peggy rita
margarida margaret margaux marge margie margo margot marguerite
margarita greta maggie margaret margaux marge margie margo margot marguerite marjorie meg rita
margaux greta maggie margaret margarida margarita marge margie margo margot marguerite marjorie meg rita
marge greta maggie margaret margarida margarita margaux margie margo margot marguerite marjorie meg peggy rita
margie greta maggie margaret margarida margarita margaux marge margo margot marguerite marjorie meg peggy rita
margo greta maggie margaret margarida margarita margaux marge margie margot marguerite marjorie meg rita
margot greta maggie margaret margarida margarita margaux marge margie margo marguerite marjorie meg peggy rita
marguerite greta maggie margaret margarida margarita margaux marge margie margo margot marjorie meg rita
maria mariah mariam marian marianne marie marion marisa marisela marisol marissa marla marlene mary maryam maryann miriam
mariah maria mariam marian marie marisa marissa marla marlene mary
mariam maria mariah marian marianne marie marion marisa marissa marla marlene mary maryam maryann miriam
marian maria mariah mariam marianne marie marion marisa marissa marla marlene mary maryam maryann miriam
marianne maria mariam marian marion mary maryam maryann miriam
marie maria mariah mariam marian marieke marilyn marisa marissa marla marlene mary maryann
marieke marie marilyn
marilyn marie marieke
marion maria mariam marian marianne marisa marisela marisol mary maryam maryann miriam
marisa maria mariah mariam marian marie marion marisela marisol marissa marla marlene mary
marisela maria marion marisa marisol marissa
marisol maria marion marisa marisela marissa
marissa maria mariah mariam marian marie marisa marisela marisol marla marlene mary
marjorie greta maggie margaret margarita margaux marge margie margo margot marguerite meg rita
mark marc marcelo marco marcus marek marko markus
marko marc marcelo marco marcus marek mark markus
markus marc marcelo marco marcus marek mark marko marlin marlon
marla maria mariah mariam marian marie marisa marissa marlene mary maryam
marlene maria mariah mariam marian marie marisa marissa marla mary maryam
marlin markus marlon
marlon markus marlin
marsha marcella marcia
marta martha
martha marta
martin marty
marty martin
mary maria mariah mariam marian marianne marie marion marisa marissa marla marlene maryam maryann maura miriam moira
maryam maria mariam marian marianne marion marla marlene mary maryann miriam
maryann maria mariam marian marianne marie marion mary maryam miriam
mateo mateusz matt matteo mattias
mateusz mateo matt matteo mattias
mathew mathieu matthew matthias
mathieu mathew matthew matthias
matilda mattie maud tilda tilly
matt mateo mateusz matteo mattias
matteo mateo mateusz matt mattias
matthew mathew mathieu matthias
matthias mathew mathieu matthew
mattias mateo mateusz matt matteo
mattie madison matilda maud tilly
maud madison matilda mattie tilly
maura mary maureen moira
maureen maura
maurice mauricio mauro moritz morris
mauricio maurice mauro moritz morris
mauro maurice mauricio moritz morris
max maxim maximilian maximus maxwell
maxim max maximilian maximus
maximilian max maxim maximus
maximus max maxim maximilian
maxwell max
meagan megan
meg greta maggie margaret margarita margaux marge margie margo margot marguerite marjorie peggy rita
megan meagan meghan
meghan megan
mel melanie melinda melissa melvin
melanie mel melinda
melinda mel melanie melissa
melissa mel melinda
melvin mel
merle merrill meryl
merrill merle meryl
meryl merle merrill
michael mick mickey micky miguel mikael mike
michaela makayla mickey mikaela mikayla
michal micheal
micheal michal michel michele mitch
michel micheal michele mitch
michele micheal michel
mick michael mickey micky miguel mihkel mika mikael mike mikel mikhail
mickey makayla michael michaela mick micky miguel mihkel mika mikael mikaela mikayla mike mikel mikhail
micky michael mick mickey miguel mihkel mika mikael mike mikel mikhail
miguel michael mick mickey micky mihkel mika mikael mike mikel mikhail
mihkel mick mickey micky miguel mika mikael mike mikel mikhail
mika mick mickey micky miguel mihkel mikael mike mikel mikhail
mikael michael mick mickey micky miguel mihkel mika mike mikel mikhail
mikaela makayla michaela mickey mikayla
mikayla makayla michaela mickey mikaela
mike michael mick mickey micky miguel mihkel mika mikael mikel mikey mikhail
mikel mick mickey micky miguel mihkel mika mikael mike mikhail
mikey michael mick mickey micky miguel mihkel mika mikael mike mikel mikhail
mikhail mick mickey micky miguel mihkel mika mikael mike mikel
miles milo
milo miles
mimi mamie
minnie minnieh
minnieh minnie
miriam maria mariam marian marianne marion mary maryam maryann
mitch micheal michel mitchell
mitchell mitch
mohamed mohammed muhammad
mohammed mohamed muhammad
moira mary maura
monica monika monique mónica
monika monica monique
monique monica monika
monte monty
monty monte
moritz maurice mauricio mauro morris
morris maurice mauricio mauro moritz
muhammad mohamed mohammed
mónica monica
nadezhda nadia nadine
nadia nadezhda nadine
nadine nadezhda nadia
nan nancy nanette
nancy nan nanette
nanette nan nancy
nasir nasser
nasser nasir
nat nate nathan nathanael nathaniel
natalia natalie natasha
natalie natalia natasha
natasha natalia natalie
nate nat nathan nathanael nathaniel
nathan nat nate nathanael nathaniel
nathanael nat nate nathan nathaniel
nathaniel nat nate nathan nathanael
neal neil nelson
ned ed eddie eddy edmund eduardo edward
neil neal nelson
nell eleanor ellen ellie
nelson neal neil
nicholas nick
nick nicholas
nicky nico nicolas nicole nicolette nikki niklas nikola nikolai nikolas
nico nicky nicoh nicolas nikolai nikolas
nicoh nico
nicolas nicky nico niklas nikola nikolai nikolas
nicole nicky nicolette nikki nikola
nicolette nicky nicole nikki nikola
nikki nicky nicole nicolette nikola
niklas nicky nicolas nikola nikolai nikolas
nikola nicky nicolas nicole nicolette nikki niklas nikolai nikolas
nikolai nicky nico nicolas niklas nikola nikolas
nikolas nicky nico nicolas niklas nikola nikolai
nino ninoh
ninoh nino
noe noah
noor nour
nora norah noreen
norah nora noreen
noreen nora norah
norm norman
norman norm
odette odile
odile odette
oliver olivier ollie
olivia ollie
olivier oliver ollie
ollie oliver olivia olivier
omar umar
omer omar
oscar ozzie
oswald ozzie ozzy
otis otto
otto otis
ozzie oscar oswald ozzy
ozzy oswald ozzie
paddy pat patrice patrick patsy
pam pamela
pamela pam
paola paula paulette pauline
paolo paul
pat paddy patrice patricia patrick patsy patty trish
patrice paddy pat patrick patsy
patricia pat patsy patty trish
patrick paddy pat patrice patsy
patsy paddy pat patrice patricia patrick patty trish
patty pat patricia patsy trish
paul paolo
paula paola paulette pauline
paulette paola paula pauline
pauline paola paula paulette
pedro pete peter petr pieter piotr
peggy greta maggie margaret marge margie margot meg rita
penelope penny
penny penelope
percival percy
percy percival
pete pedro peter petr pieter piotr
peter pedro pete petr pieter piotr
petr pedro pete peter pieter piotr
philip philippe phillip
philippe philip phillip
phillip philip philippe
pierce pierre
pierre pierce
pieter pedro pete peter petr piotr
piotr pedro pete peter petr pieter
quentin quincy quinton
quincy quentin quinton
quinton quentin quincy
rachael raquel
rachel rachelle
rachelle rachel
rafael rafe raphael
ragnar rainer
rainer ragnar
ralph raul
ramon ray raymond raymundo
randal randall randolf randolph randy
randall randal randolf randolph randy
randolf randal randall randolph randy
randolph randal randall randolf randy
randy randal randall randolf randolph
raphael rafael
raquel rachael
rasheed rashid
rashid rasheed
raul ralph
ray ramon raymond raymundo
raymond ramon ray raymundo
raymundo ramon ray raymond
rayna reyna
reba becky rebecca rebeccah rebekah
rebeca rebecca
rebecca becky reba rebeca rebeccah rebekah
rebeccah reba rebecca rebekah
rebekah becky reba rebecca rebeccah
reed reid
reid reed
reiko reikoh
reikoh reiko
reuben ruben ruby
ricardo riccardo rickey ricki ricky
riccardo ricardo rickey ricki ricky
rich dick richard richie rick
richard dick rich richie rick
richie dick rich richard rick
rick dick rich richard richie
rickey ricardo riccardo ricki ricky
ricki ricardo riccardo rickey ricky rikki
ricky ricardo riccardo rickey ricki
rihna rina
rihtu ritu
rikki ricki
riley rylee
rina rihna rinah
rita greta maggie margaret margarita margaux marge margie margo margot marguerite marjorie meg peggy
ritu rihtu
rob bob bobbie bobby robbie robert roberto robin
robbie bob bobbie bobby rob robert roberto robin robyn
robert bob bobbie bobby rob robbie roberto robin
roberto bob bobby rob robbie robert robin
robin bob bobbie bobby rob robbie robert roberto robyn
robyn bobbie bobby robbie robin
rocco rocky
rocky rocco
rod roddy roderic roderick rodney rodrigo
roddy rod roderic roderick rodney rodrigo
roderic rod roddy roderick rodrigo
roderick rod roddy roderic rodrigo
rodney rod roddy
rodolfo rudolph rudy
rodrigo rod roddy roderic roderick rudy
roland rolando
rolando roland
ron ronald ronnie ronny
ronald ron ronnie ronny
ronnie ron ronald ronny
ronny ron ronald ronnie
rosa rosanna rosannah rose rosetta
rosalind roslyn
rosanna rosa rosannah rose rosemary rosetta rosie
rosannah rosa rosanna
rose rosanna rosemary rosie
rosemary rosanna rose rosie
rosetta rosa rosanna
rosie rosanna rose rosemary
roslyn rosalind
roxanne roxie
roxie roxanne
ruben reuben ruby
rubina ruby
ruby reuben ruben rubina
rudolph rodolfo rudy
rudy rodolfo rudolph ruud
ruud rudy
rylee riley
ryo ryoh
ryoh ryo
sadie sallie sally
sallie sadie sally
sally sadie sallie
salomon sol solomon
salvador salvatore
salvatore salvador
sam samantha sami sammy samson samuel
samantha sam sammy
sami sam sammy samuel
sammy sam samantha sami samson samuel
samson sam sammy
samuel sam sami sammy
sander sandy
sandrine sandy
sandy sander sandrine
sara sarah
sarah sara
scott scotty
scotty scott
sergei sergey sergio
sergey sergei sergio
sergio sergei sergey
shahri shari
shan shana
shana shan
shane shaun shawn
shari shahri sharih
sharih shari
sharlene charlene charlie
shaun shane shawn
shauna shawna
shawn shane shaun
shawna shauna
shay shea
shayla sheila
shea shay
sheila shayla
sheri cherie cheryl sherri sherry
sherri cherie cheryl sheri sherry
sherry cherie cheryl sheri sherri
shira shirah
sid sidney sydney
sidney sid sydney
sihti siti
silvia sylvia sylvie
simon simone
simona simone
simone simon simona
sissy cecilia
siti sihti
sofia sophia sophie
sohta sota
sol salomon solomon
solomon salomon sol
sonia sonya
sonya sonia
soo sooh
sooh soo
sophia sofia sophie
sophie sofia sophia
sota sohta
stacey stacy
stacy stacey
stan stanley
stanley stan
stefan stefano steph stephan stephen steve steven stevie
stefanie steph stephanie stevie
stefano stefan steph stephan stephen steve steven stevie
steph stefan stefanie stefano stephan stephanie stephen steve steven stevie
stephan stefan stefano steph stephen steve steven stevie
stephanie stefanie steph stevie
stephen stefan stefano steph stephan steve steven stevie
steve stefan stefano steph stephan stephen steven stevie
steven stefan stefano steph stephan stephen steve stevie
stevie stefan stefanie stefano steph stephan stephanie stephen steve steven
stewart stu stuart
stu stewart stuart
stuart stewart stu
sue susan susanna susannah susie suzanne suzy
susan sue susanna susannah susie suzanne suzy
susanna sue susan susannah susie suzanne suzy
susannah sue susan susanna susie
susie sue susan susanna susannah susieh suzanne suzy
susieh susie
suzanne sue susan susanna susie suzy
suzy sue susan susanna susie suzanne
sydney sid sidney
sylvia silvia sylvie
sylvie silvia sylvia
tabatha tabitha
tabitha tabatha
tad tadeo
tadeo tad
tahlia talia
talia tahlia
tamar tamara tamarah tammy tamra
tamara tamar tammy tamra
tamika tammy
tammy tamar tamara tamika tamra
tamra tamar tamara tammy
tamsen tamsin tamson tamsyn
tamsin tamsen tamson tamsyn
tamson tamsen tamsin tamsyn
tamsyn tamsen tamsin tamson
tara tarah
tarah tara
taro taroh
taroh taro
ted teddy theodore
teddy ted theodore
terence terrance terrence
teresa tereza terri trace traci tracy
tereza teresa terri trace traci tracy
terrance terence terrence
terrence terence terrance
terri teresa tereza trace traci tracy
terry tess theresa
tess terry theresa
tessa tessie
tessie tessa
theodore ted teddy
theresa terry tess
thomas tom tomas tomasz tommaso tommie tommy
tiffani tiffany
tiffany tiffani
tilda matilda tilly
tilly matilda mattie maud tilda
tim timmy timothy
timmy tim timothy
timothy tim timmy
tina chris chrissy christina christine christy cristina kirsten kris kristen kristin kristina
tobias tobin toby
tobin tobias toby
toby tobias tobin
tohri tori
tom thomas tomas tomasz tommaso tommie tommy
tomas thomas tom tomasz tommaso tommie tommy
tomasz thomas tom tomas tommaso tommie tommy
tommaso thomas tom tomas tomasz tommie tommy
tommie thomas tom tomas tomasz tommaso tommy
tommy thomas tom tomas tomasz tommaso tommie
tomoko tomokoh
tomokoh tomoko
toni tonia tonya
tonia toni tonya
tony anthony antoine anton antonio
tonya toni tonia
tori tohri
trace teresa tereza terri traci tracy
traci teresa tereza terri trace tracy
tracy teresa tereza terri trace traci
trent trenton
trenton trent
tricia trisha
trish pat patricia patsy patty
trisha tricia
tristan tristen
tristen tristan
umar omar
ursula urszula
urszula ursula
val valentin valentina valentine valentino valeria valerie valerio
valentin val
valentina val valentine
valentine val valentina valentino
valentino val valentine
valeria val valerie
valerie val valeria
valerio val
van vance
vance van
vern vernon
vernon vern
vic vicki vickie vicky victor victoria viktor
vicki vic vickie vicky victoria
vickie vic vicki vicky victoria
vicky vic vicki vickie victoria
victor vic viktor
victoria vic vicki vickie vicky
viktor vic victor
vincent vincenzo
vincenzo vincent
virgie virginia
virginia virgie
vivian vivienne
vivienne vivian
wallace wally
wally wallace walt walter
walt wally walter
walter wally walt
wesley westley
westley wesley
wilhelmina willa willie wilma
will bill billy willem william willie willis willy wilmer wilson
willa wilhelmina willie wilma
willem bill billy will william willie willis willy wilmer wilson
william bill billy will willem willie willis willy wilmer wilson
willie bill billy wilhelmina will willa willem william willis willy wilma wilmer wilson
willis will willem william willie willy wilmer wilson
willy bill billy will willem william willie willis wilmer wilson
wilma wilhelmina willa willie
wilmer will willem william willie willis willy wilson
wilson will willem william willie willis willy wilmer
winifred winnie
winnie winifred
woodrow woody
woody woodrow
xander zander
yan yannis
yannis yan
yehmi yemi
yemi yehmi
yohko yoko yokoh
yoko yohko yokoh
yokoh yohko yoko
yosef yousef youssef yusuf
yousef yosef youssef yusuf
youssef yosef yousef yusuf
yuri yurih
yurih yuri
yusuf yosef yousef youssef
zacarias zachary
zach zachariah zachary zack
zachariah zach zachary zack
zachary zach zachariah zack
zack zach zachariah zachary
zahrah zara zaria
zander xander
zara zahrah zaria
zaria zahrah zara
zoe zoey zoë
zoey zoe zoë
zoë zoe zoey
zuri zurih
zurih zuri
`

var nameVariants = func() map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(nameVariantTable), "\n") {
		names := strings.Fields(line)
		for i := range names {
			names[i] = strings.ReplaceAll(names[i], "_", " ")
		}
		for _, existing := range names[1:] {
			out[names[0]+"\x00"+existing] = true
		}
	}
	return out
}()

// nameVariantOf reports an existing name the candidate's dictionary entry lists.
func nameVariantOf(candidate, existing string) bool {
	return nameVariants[candidate+"\x00"+existing]
}

// nameAliasKeys are dictionary names whose match key comes from an alias and
// not from their own spelling: natively they find no name by Jaro-Winkler
// under their own key ("Kate" does not find "Katte", "Helen" does not find
// "Helena"). The value is the alias key where a row shows it ("Leo" finds
// "Leona" under the key of "Leon") and empty where no row does.
var nameAliasKeys = map[string]string{
	"anita":   "",
	"becky":   "",
	"bella":   "",
	"bert":    "",
	"bessie":  "",
	"beth":    "",
	"betsy":   "",
	"betty":   "",
	"bill":    "",
	"billy":   "",
	"bob":     "",
	"bobbie":  "",
	"bobby":   "",
	"chuck":   "",
	"delores": "",
	"dick":    "",
	"dolly":   "",
	"emma":    "",
	"fanny":   "",
	"gail":    "",
	"gene":    "",
	"gina":    "",
	"greta":   "",
	"gus":     "",
	"hallie":  "",
	"hattie":  "",
	"helena":  "",
	"hetty":   "",
	"jane":    "",
	"janet":   "",
	"jeanne":  "",
	"jo":      "",
	"jo ann":  "",
	"joe":     "",
	"kate":    "",
	"kay":     "",
	"kit":     "",
	"kitty":   "",
	"leo":     "LN",
	"libby":   "",
	"lila":    "",
	"lina":    "",
	"linda":   "",
	"liz":     "",
	"lottie":  "",
	"lou":     "",
	"louie":   "LS",
	"lucinda": "",
	"lula":    "",
	"maggie":  "",
	"mandy":   "",
	"meg":     "",
	"mona":    "",
	"myra":    "",
	"nat":     "",
	"nate":    "",
	"ned":     "",
	"nell":    "",
	"nick":    "",
	"ola":     "",
	"peggy":   "",
	"rick":    "",
	"rita":    "",
	"roberta": "",
	"sadie":   "",
	"sandra":  "",
	"ted":     "",
	"teddy":   "",
	"terry":   "",
	"tess":    "",
	"tilly":   "",
	"tina":    "",
	"tony":    "",
	"trish":   "",
	"trudy":   "",
	"zeke":    "",
}

// nameKeptApart are pairs the rows keep apart that the tables above do not
// explain.
var nameKeptApart = map[[2]string]bool{
	{"ray", "rayh"}: true,
	{"ray", "rayw"}: true,
	{"sue", "sueh"}: true,
	{"sue", "suew"}: true,
	{"ty", "tyh"}:   true,
	{"ty", "tyw"}:   true,
}

var companyAcronyms = map[string]string{
	"amd":  "advanced micro devices",
	"bmw":  "bayerische motoren werke",
	"ge":   "general electric",
	"gm":   "general motors",
	"hp":   "hewlett packard",
	"ibm":  "international business machines",
	"kfc":  "kentucky fried chicken",
	"nasa": "national aeronautics space administration",
	"pg&e": "pacific gas electric",
	"pge":  "pacific gas electric",
	"ups":  "united parcel service",
}

// companyWordPairs are the company-name words the native rows match to each
// other when only that word differs, with the native match confidence in each
// direction (forward is the first word as candidate, reverse the second).
// They cover plural, spelling and vowel variants (Traders/Trader,
// Verizon/Verizone, Nordstrom/Nordstram); the rows also keep near neighbours
// apart (Partners/Partner, Marine/Marin, Horizon/Horizan), so no
// generalization beyond a doubled letter is applied. Pairs the native rows
// match in one direction only are left out.
var companyWordPairs = []struct {
	a, b             string
	forward, reverse float64
}{
	{"advisor", "advisors", 100, 100},
	{"airline", "airlines", 100, 100},
	{"america", "americas", 100, 100},
	{"caterpilar", "caterpiller", 98, 97},
	{"caterpillar", "caterpiller", 98, 97},
	{"davidsan", "davidson", 100, 100},
	{"goldman", "goldmann", 100, 100},
	{"guitar", "guitars", 100, 100},
	{"holding", "holdings", 100, 100},
	{"importer", "importers", 100, 100},
	{"industri", "industrial", 100, 100},
	{"industri", "industrials", 100, 100},
	{"industri", "industrie", 100, 100},
	{"industri", "industries", 100, 100},
	{"industri", "industry", 100, 100},
	{"industrial", "industrials", 100, 100},
	{"industrial", "industrie", 96, 96},
	{"industrial", "industries", 96, 97},
	{"industrial", "industry", 94, 96},
	{"industrials", "industries", 100, 100},
	{"industrie", "industries", 100, 100},
	{"industrie", "industry", 94, 96},
	{"industries", "industry", 96, 97},
	{"initec", "initeck", 95, 95},
	{"initech", "initeche", 100, 100},
	{"kinetic", "kinetick", 100, 100},
	{"kinetic", "kinetik", 100, 100},
	{"kinetick", "kinetik", 100, 100},
	{"lockhead", "lockheed", 100, 100},
	{"machine", "machines", 100, 100},
	{"margie", "margies", 100, 100},
	{"market", "markets", 100, 100},
	{"momentem", "momentom", 100, 100},
	{"momentem", "momentum", 100, 100},
	{"momentom", "momentum", 100, 100},
	{"morningstar", "morningstarr", 100, 100},
	{"nordstram", "nordstrom", 97, 97},
	{"nordstram", "nordstrum", 97, 97},
	{"nordstrom", "nordstrum", 97, 97},
	{"northwind", "northwinds", 100, 100},
	{"packard", "packards", 100, 100},
	{"proseware", "prosewear", 98, 97},
	{"stanlee", "stanlei", 100, 100},
	{"stanlee", "stanley", 100, 100},
	{"stanlei", "stanley", 100, 100},
	{"stanlei", "stanleyy", 98, 98},
	{"stanley", "stanleyy", 100, 100},
	{"starbucks", "starbucs", 96, 96},
	{"starbucks", "starbux", 95, 93},
	{"starbucs", "starbux", 98, 97},
	{"studio", "studios", 100, 100},
	{"trader", "traders", 100, 100},
	{"verizon", "verizone", 100, 100},
}
