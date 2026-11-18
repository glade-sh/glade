@IsTest private class SetterRead67 {
 class SameBox { public String label; public String seen; public SameBox backing; public Integer getters=0; public SameBox current {get {getters++; return backing;} set {backing=value; seen=current.label;}} }
 class DirectBox { public String label; public String seen; public DirectBox backing; public Integer getters=0; public DirectBox current {get {getters++; return backing;} set {backing=value; seen=backing.label;}} }
 class OtherBox { public String label; public String seen; public OtherBox backing; public Integer getters=0; public OtherBox other {get {getters++; return backing;}} public OtherBox current {get {return backing;} set {backing=value; seen=other.label;}} }
 class SafeBox { public String label; public String seen; public SafeBox backing; public Integer getters=0; public SafeBox current {get {getters++; return backing;} set {backing=value; seen=current==null ? 'NULL' : current.label;}} }
 @IsTest static void sameMember() {SameBox child=new SameBox();child.label='Synthetic';SameBox parent=new SameBox();parent.current=child;System.assertEquals('Synthetic',parent.seen);System.assertEquals(1,parent.getters);}
 @IsTest static void directBacking() {DirectBox child=new DirectBox();child.label='Synthetic';DirectBox parent=new DirectBox();parent.current=child;System.assertEquals('Synthetic',parent.seen);System.assertEquals(0,parent.getters);}
 @IsTest static void otherGetter() {OtherBox child=new OtherBox();child.label='Synthetic';OtherBox parent=new OtherBox();parent.current=child;System.assertEquals('Synthetic',parent.seen);System.assertEquals(1,parent.getters);}
 @IsTest static void sameSafeObserve() {SafeBox child=new SafeBox();child.label='Synthetic';SafeBox parent=new SafeBox();parent.current=child;System.assertEquals('OBS',parent.seen+'|getters='+parent.getters);}
 @IsTest static void nullReject() {OtherBox parent=new OtherBox();parent.current=null;}
}
