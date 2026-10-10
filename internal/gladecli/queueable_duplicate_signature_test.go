package gladecli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunExecQueueableDuplicateSignatureWithProjectRuntime(t *testing.T) {
	// A38 T002-T007/T038/T039: factory and explicit System Builder native controls.
	tests := map[string]string{
		"qualified builder": `QueueableDuplicateSignature sig = QueueableDuplicateSignature.builder().addString('job').addInteger(42).addId('001000000000001AAA').build();
System.assert(sig != null);
System.assert('\'job\'_42_001000000000001'.equals(sig.toString()));
System.assert('\'job\'_42_001000000000001'.equals(String.valueOf(sig)));`,
		"builder size": `System.QueueableDuplicateSignature.Builder builder = QueueableDuplicateSignature.builder();
System.assertEquals(0, builder.getSize());
System.assertEquals(32, builder.getMaxSize());
System.assertEquals(32, builder.getRemainingSize());
builder.addString('nightly');
builder.addInteger(7);
builder.addId('001000000000001AAA');
System.assertEquals(26, builder.getSize());
System.assertEquals(6, builder.getRemainingSize());
QueueableDuplicateSignature signature = builder.build();
System.assert(signature != null);
System.assert('\'nightly\'_7_001000000000001'.equals(signature.toString()));
System.assert('\'nightly\'_7_001000000000001'.equals(String.valueOf(signature)));`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", ".", "--json", source}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("queueable duplicate signature execution failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if strings.Contains(stderr.String(), "NullPointerException") || strings.Contains(stderr.String(), "expected <") {
				t.Fatalf("queueable duplicate signature emitted runtime failure: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}
