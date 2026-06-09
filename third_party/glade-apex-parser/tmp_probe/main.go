package main

import (
    "fmt"

    ts "github.com/tree-sitter/go-tree-sitter"
    "github.com/glade-sh/apex-parser/internal/tsapex"
)

func walk(node *ts.Node, source string, depth int) {
    for i := 0; i < int(depth); i++ {
        fmt.Print("  ")
    }
    start := node.StartByte()
    end := node.EndByte()
    raw := source[start:end]
    preview := string(raw)
    if len(preview) > 70 {
        preview = preview[:70]
    }
    fmt.Printf("%s [%d:%d] %q\n", node.Kind(), start, end, preview)
    for i := uint(0); i < node.ChildCount(); i++ {
        child := node.Child(i)
        if child != nil && child.IsNamed() {
            walk(child, source, depth+1)
        }
    }
}

func main() {
    p := ts.NewParser()
    if err := p.SetLanguage(tsapex.GetLanguage()); err != nil {
        panic(err)
    }
    source := `public class Probe {
  public void run(List<SObject> rows) {
    SObjectType t = Account.SObjectType;
    Schema.SObjectType st = Account.SObjectType;
    t.getDescribe();
    Account.SObjectType.getDescribe();
    Account.SObjectType.getDescribe();
    Database.DatabaseInfo.getInstance();
    Database.getQueryLocator('select id from Account');
  }
}`
    tree := p.Parse([]byte(source), nil)
    root := tree.RootNode()
    walk(root, source, 0)
    tree.Close()
    p.Close()
}
