System.assertEquals('ready', new ReceiverControl().instanceValue());
System.assertEquals('ready', ReceiverControl.staticValue());
ReceiverDependency receiver = null;
Boolean caught = false;
try { receiver.value(); } catch (NullPointerException e) { caught = true; }
System.assertEquals(true, caught);
