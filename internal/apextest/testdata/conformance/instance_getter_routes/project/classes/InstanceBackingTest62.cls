@IsTest private class InstanceBackingTest62 {
 class Probe { public Integer visits=0; public Probe peer; public String item { get { visits++; if (peer!=null) { String currentValue=peer.item; } return 'ok'; } set; } }
 @IsTest static void otherInstanceBacking() { Probe first=new Probe(); Probe second=new Probe(); first.peer=second; System.assertEquals('ok',first.item); System.assertEquals(1,first.visits); System.assertEquals(0,second.visits); }
 @IsTest static void referenceCycle() { Probe first=new Probe(); Probe second=new Probe(); first.peer=second; second.peer=first; System.assertEquals('ok',first.item); System.assertEquals(1,first.visits); System.assertEquals(0,second.visits); }
}
