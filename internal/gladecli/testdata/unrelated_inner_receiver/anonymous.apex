ProbeConsumer.Request r = new ProbeConsumer.Request();
System.assertEquals(0, r.run());
System.assertEquals(0, r.explicitSystem());
System.assertEquals(41, r.qualified());
System.assertEquals(41, new ProbeContainer.Reader().own());
