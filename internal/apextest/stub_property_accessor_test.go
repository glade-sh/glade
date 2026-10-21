package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// The Stub API does not mock properties. A property read on a stub runs the
// declared accessor, and the provider sees only the non-private method that
// accessor calls on the stub. A recorder that prepares an answer for the last
// call it saw while stubbing must find that same call when the property is
// read again.
func TestRunStubPropertyAccessorReachesProviderOnlyThroughMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion": "62.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SlotBase.cls"), `public virtual class SlotBase {
    public virtual Object readSlot(String key) { return 'declared-' + key; }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SlotTarget.cls"), `public virtual class SlotTarget extends SlotBase {
    public String Label { get { return (String)readSlot('label'); } }
    public String Plain { get; set; }
    public override virtual Object readSlot(String key) { return super.readSlot(key); }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/SlotProof.cls"), `@IsTest private class SlotProof {
    private class ProofException extends Exception {}
    private class Recorder implements System.StubProvider {
        public Boolean stubbing = false;
        public String last;
        public Set<String> prepared = new Set<String>();
        public Map<String, Object> answers = new Map<String, Object>();
        public List<String> calls = new List<String>();
        public Object handleMethodCall(Object stubbedObject, String stubbedMethodName, Type returnType,
            List<Type> listOfParamTypes, List<String> listOfParamNames, List<Object> listOfArgs) {
            String key = stubbedMethodName + listOfArgs;
            calls.add(key);
            if (stubbing) {
                prepared.add(key);
                last = key;
                return null;
            }
            if (prepared.contains(key) && !answers.containsKey(key)) {
                throw new ProofException('prepared without answer: ' + key);
            }
            return answers.get(key);
        }
        public void thenReturn(Object value) { answers.put(last, value); }
    }
    @IsTest static void propertyReadUsesTheStubbedMethodAnswer() {
        Recorder recorder = new Recorder();
        SlotTarget stub = (SlotTarget)Test.createStub(SlotTarget.class, recorder);
        recorder.stubbing = true;
        Object ignored = stub.Label;
        recorder.thenReturn('answer');
        recorder.stubbing = false;
        System.assertEquals('answer', stub.Label);
        System.assertEquals(new List<String>{ 'readSlot(label)', 'readSlot(label)' }, recorder.calls);
    }
    @IsTest static void autoPropertyIsNotSentToProvider() {
        Recorder recorder = new Recorder();
        SlotTarget stub = (SlotTarget)Test.createStub(SlotTarget.class, recorder);
        System.assertEquals(null, stub.Plain);
        stub.Plain = 'kept';
        System.assertEquals('kept', stub.Plain);
        System.assertEquals(0, recorder.calls.size());
    }
}`)
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("stub property accessor contract failed: %s", data)
	}
}
