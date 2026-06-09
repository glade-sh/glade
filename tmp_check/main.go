package main

import (
    "fmt"
    "github.com/glade-sh/glade/internal/soql"
)

func main() {
    queries := []string{
        "SELECT COUNT() FROM Account",
        "SELECT count(Id) FROM Account",
        "SELECT COUNT(Id) FROM Error_Log__c",
        "SELECT Id, Name FROM Account",
        "SELECT Id, COUNT(Id) FROM Account GROUP BY Name",
    }
    for _, q := range queries {
        parsed, err := soql.Parse(q)
        if err != nil {
            fmt.Printf("%s -> err=%v\n", q, err)
            continue
        }
        fmt.Printf("%q\n", q)
        fmt.Printf("  Count=%v HasLimit=%v Limit=%d\n", parsed.Count, parsed.HasLimit, parsed.Limit)
        fmt.Printf("  Aggregates=%+v\n", parsed.Aggregates)
        fmt.Printf("  Fields=%v\n", parsed.Fields)
    }
}
