@IsTest public class ElementProbeTest {
 @IsTest static void PE01_deliver() { ElementProbe.observe('PE01','create','deliver'); }
 @IsTest static void PE01_stop() { ElementProbe.observe('PE01','create','stop'); }
 @IsTest static void PE02_deliver() { ElementProbe.observe('PE02','reject','deliver'); }
 @IsTest static void PE02_stop() { ElementProbe.observe('PE02','reject','stop'); }
 @IsTest static void PE03_deliver() { ElementProbe.observe('PE03','update','deliver'); }
 @IsTest static void PE03_stop() { ElementProbe.observe('PE03','update','stop'); }
 @IsTest static void PE04_deliver() { ElementProbe.observe('PE04','delete','deliver'); }
 @IsTest static void PE04_stop() { ElementProbe.observe('PE04','delete','stop'); }
 @IsTest static void PE05_deliver() { ElementProbe.observe('PE05','loop','deliver'); }
 @IsTest static void PE05_stop() { ElementProbe.observe('PE05','loop','stop'); }
 @IsTest static void PE07_deliver() { ElementProbe.observe('PE07','order','deliver'); }
 @IsTest static void PE07_stop() { ElementProbe.observe('PE07','order','stop'); }
 @IsTest static void PE10_deliver() { ElementProbe.observe('PE10','batch','deliver'); }
 @IsTest static void PE10_stop() { ElementProbe.observe('PE10','batch','stop'); }
 @IsTest static void PE11_deliver() { ElementProbe.observe('PE11','user','deliver'); }
 @IsTest static void PE11_stop() { ElementProbe.observe('PE11','user','stop'); }
 @IsTest static void PE12_deliver() { ElementProbe.observe('PE12','chatter','deliver'); }
 @IsTest static void PE12_stop() { ElementProbe.observe('PE12','chatter','stop'); }
 @IsTest static void PE13_deliver() { ElementProbe.observe('PE13','apex','deliver'); }
 @IsTest static void PE13_stop() { ElementProbe.observe('PE13','apex','stop'); }
}
