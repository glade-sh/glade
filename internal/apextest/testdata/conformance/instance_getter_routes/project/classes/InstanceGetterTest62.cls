@IsTest private class InstanceGetterTest62 {
 class Probe {
  public Integer peerVisits=0, helperVisits=0, directVisits=0;
  public Probe peer;
  public String peerValue { get { peerVisits++; if (peer!=null) { String currentValue=peer.peerValue; } return 'peer'; } set; }
  public String helperValue { get { helperVisits++; if (helperVisits==1) { String currentValue=readHelper(); } return 'helper'; } set; }
  public String readHelper() { return helperValue; }
  public String recursiveValue { get { return recursiveHelper(); } set; }
  public String recursiveHelper() { return recursiveValue; }
  public String direct { get { directVisits++; if (directVisits==1) { String currentValue=direct; } return 'direct'; } set; }
 }
 @IsTest static void otherInstance() { Probe first=new Probe(); Probe second=new Probe(); first.peer=second; System.assertEquals('peer',first.peerValue); System.assertEquals(1,first.peerVisits); System.assertEquals(1,second.peerVisits); }
 @IsTest static void crossInstanceCycle() { Probe first=new Probe(); Probe second=new Probe(); first.peer=second; second.peer=first; String currentValue=first.peerValue; }
 @IsTest static void finiteHelper() { Probe obj=new Probe(); System.assertEquals('helper',obj.helperValue); System.assertEquals(2,obj.helperVisits); }
 @IsTest static void recursiveHelper() { Probe obj=new Probe(); String currentValue=obj.recursiveValue; }
 @IsTest static void directControl() { Probe obj=new Probe(); System.assertEquals('direct',obj.direct); System.assertEquals(1,obj.directVisits); }
}
