package sema

import (
	"reflect"
	"testing"
)

func TestReceiverTriviaScan(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         []receiverTriviaKind
	}{
		{"bound", "thing.\nmember", []receiverTriviaKind{receiverBoundBreak}},
		{"bound comment", "thing./* comment */member", []receiverTriviaKind{receiverBoundBreak}},
		{"this keyword", "this.\nmember", []receiverTriviaKind{receiverBoundBreak}},
		{"super keyword", "super.\nmember", []receiverTriviaKind{receiverBoundBreak}},
		{"trigger context", "Trigger.\nnew", []receiverTriviaKind{receiverBoundBreak}},
		{"free", "thing \n. member", []receiverTriviaKind{receiverFreeName}},
		{"free comment", "thing/* comment */.member", []receiverTriviaKind{receiverFreeName}},
		{"safe navigation", "thing?.\nmember", nil},
		{"call result", "thing().\nmember", nil},
		{"index result", "things[0].\nmember", nil},
		{"literal", "'thing'.\nlength()", nil},
		{"strings comments and queries", "String s = 'thing. member'; /* x. y */ // x. y\nList<Account> a = [SELECT Owner.Name FROM Account];", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			breaks := scanReceiverTrivia(tc.source)
			var got []receiverTriviaKind
			for _, b := range breaks {
				got = append(got, b.kind)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
