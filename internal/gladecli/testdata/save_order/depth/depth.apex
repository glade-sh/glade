String obs='test:'+Test.isRunningTest()+'|'+DepthTrace.probe(15)+'|'+DepthTrace.probe(16)+'|'+DepthTrace.probe(17);
throw new DepthResultException(obs);
