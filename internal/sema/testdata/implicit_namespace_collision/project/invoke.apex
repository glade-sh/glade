NamespaceMemberProbe probe = new NamespaceMemberProbe();
probe.ns = new NamespaceMemberContainer();
probe.ns.service = new NamespaceMemberService();
probe.invoke();
System.assertEquals(1, probe.ns.service.calls);
