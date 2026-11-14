CrProbe x = new CrProbe();
List<CrProbe> xs = new List<CrProbe>{ x };
System.assertEquals('ok', xs[0].
instanceValue());
System.assertEquals('ok', xs[0]
.instanceValue());
System.assertEquals(2, 'ab'.
length());
System.assertEquals(2, 'ab'
.length());
System.assertEquals('call', x.child
.call());
System.assertEquals('call', x
.child.call());
System.assertEquals('ok', CrProbe.value ());
