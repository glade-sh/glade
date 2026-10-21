package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Interface callbacks select their concrete implementation. Duplicate project
// declarations must retain that same documented dispatch behavior.
func TestDuplicateDeclarationInterfaceCallback(t *testing.T) {
	for _, namespace := range []string{"", "sample"} {
		t.Run(namespace, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+namespace+`","sourceApiVersion":"66.0","packageDirectories":[{"path":"package","default":true},{"path":"force-app","default":false}]}`)
			helper := `public class DeliveryHooks {
   public interface Error {Exception getException();}
   public interface Logger {void log(Error error);}
   @TestVisible private static Logger sink {get{return sink;} private set;}
   private class Payload implements Error {public Exception getException(){return null;}}
   public static void emit(){sink.log(new Payload());}
  }`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DeliveryHooks.cls"), helper)
			writeFile(t, filepath.Join(root, "package/main/default/classes/DeliveryHooks.cls"), strings.ReplaceAll(strings.Replace(helper, "public class", "global class", 1), "public interface", "global interface"))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DeliveryProbe.cls"), `@IsTest private class DeliveryProbe {
   private class Recorder implements DeliveryHooks.Logger {public DeliveryHooks.Error seen;public Integer calls=0;public void log(DeliveryHooks.Error error){seen=error;calls++;}}
   @IsTest static void observe(){Recorder receiver=new Recorder();DeliveryHooks.sink=receiver;DeliveryHooks.emit();System.assertEquals(1,receiver.calls);}
  }`)
			writeFile(t, filepath.Join(root, "package/main/default/classes/CompleteDeliveryProbe.cls"), `@IsTest private class CompleteDeliveryProbe {
 private class Recorder implements DeliveryHooks.Logger {public DeliveryHooks.Error seen;public Integer calls=0;public void log(DeliveryHooks.Error error){seen=error;calls++;}}
 @IsTest static void observe(){Recorder receiver=new Recorder();DeliveryHooks.sink=receiver;DeliveryHooks.emit();System.assertEquals(1,receiver.calls);}
}`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"DeliveryProbe", "CompleteDeliveryProbe"}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
				t.Fatalf("callback run: %#v", run)
			}
		})
	}
}
