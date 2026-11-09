System.QueueableDuplicateSignature.Builder b = QueueableDuplicateSignature.builder();
System.assertEquals(0,b.getSize());
System.assertNotEquals(null,b.addString('job').addInteger(42).addId('001000000000001AAA').build());
System.debug('B5_BUILDER_OK');
