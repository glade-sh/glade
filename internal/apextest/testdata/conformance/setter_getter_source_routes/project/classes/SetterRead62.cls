@IsTest private class SetterRead62 {
 class SameBox { public String label; public String seen; public SameBox backing; public Integer getters=0; public SameBox current {get {getters++; return backing;} set {backing=value; seen=current.label;}} }
 class DirectBox { public String label; public String seen; public DirectBox backing; public Integer getters=0; public DirectBox current {get {getters++; return backing;} set {backing=value; seen=backing.label;}} }
 class OtherBox { public String label; public String seen; public OtherBox backing; public Integer getters=0; public OtherBox other {get {getters++; return backing;}} public OtherBox current {get {return backing;} set {backing=value; seen=other.label;}} }
 class SafeBox { public String label; public String seen; public SafeBox backing; public Integer getters=0; public SafeBox current {get {getters++; return backing;} set {backing=value; seen=current==null ? 'NULL' : current.label;}} }
 class QualifiedBox {public String label; public String seen; public QualifiedBox backing; public Integer getters=0; public QualifiedBox current {get {getters++;return backing;} set {backing=value;seen=this.current.label;}}}
 class HelperBox {public String label; public String seen; public HelperBox backing; public Integer getters=0; public HelperBox current {get {getters++;return backing;} set {backing=value;seen=helper().label;}} public HelperBox helper(){return current;}}
 @IsTest static void qualifiedSame() {QualifiedBox c=new QualifiedBox();c.label='Synthetic';QualifiedBox p=new QualifiedBox();p.current=c;System.assertEquals('Synthetic',p.seen);}
 @IsTest static void sameHelper() {HelperBox c=new HelperBox();c.label='Synthetic';HelperBox p=new HelperBox();p.current=c;System.assertEquals('Synthetic',p.seen);System.assertEquals(1,p.getters);}
 @IsTest static void sameMember() {SameBox child=new SameBox();child.label='Synthetic';SameBox parent=new SameBox();parent.current=child;System.assertEquals('Synthetic',parent.seen);System.assertEquals(1,parent.getters);}
 @IsTest static void directBacking() {DirectBox child=new DirectBox();child.label='Synthetic';DirectBox parent=new DirectBox();parent.current=child;System.assertEquals('Synthetic',parent.seen);System.assertEquals(0,parent.getters);}
 @IsTest static void otherGetter() {OtherBox child=new OtherBox();child.label='Synthetic';OtherBox parent=new OtherBox();parent.current=child;System.assertEquals('Synthetic',parent.seen);System.assertEquals(1,parent.getters);}
 @IsTest static void sameSafeObserve() {SafeBox child=new SafeBox();child.label='Synthetic';SafeBox parent=new SafeBox();parent.current=child;System.assertEquals('NULL|getters=0',parent.seen+'|getters='+parent.getters);}
 @IsTest static void nullReject() {OtherBox parent=new OtherBox();parent.current=null;}
}
