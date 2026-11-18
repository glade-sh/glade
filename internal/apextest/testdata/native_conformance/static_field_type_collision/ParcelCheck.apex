@IsTest private class ParcelCheck { @IsTest static void verify() { System.assertEquals('ab', Parcel.Builder.build('b').value()); } }
