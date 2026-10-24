@IsTest private class RejectedFinalizerCall { @IsTest static void observe() { System.Finalizer receiver=new ConcreteFinalizerField.Collector(); receiver.add(1); } }
