import { describe, expect, test } from "vitest"

import { highlightApex } from "./apex-highlight"

describe("highlightApex", () => {
  test("marks Apex-native tokens with Apex-specific classes", () => {
    const html = highlightApex(`@AuraEnabled(cacheable=true)
public with sharing class AccountPlayground {
  public static List<Account> findByName(String term) {
    return [SELECT Id, Name FROM Account WHERE Name LIKE :term LIMIT 1];
  }
}`)

    expect(html).toContain('<span class="token annotation-name">@AuraEnabled</span>')
    expect(html).toContain('<span class="token annotation-attr">cacheable</span>')
    expect(html).toContain('<span class="token system-type">List</span>')
    expect(html).toContain('<span class="token class-name">Account</span>')
    expect(html).toContain('<span class="token method-declaration">findByName</span>')
    expect(html).toContain('<span class="token soql-keyword">SELECT</span>')
    expect(html).toContain('<span class="token sobject-field">Name</span>')
    expect(html).toContain('<span class="token sobject-name">Account</span>')
    expect(html).toContain('<span class="token bind-variable">:term</span>')
  })

  test("escapes source text before wrapping tokens", () => {
    const html = highlightApex("String tag = '<script>';")

    expect(html).toContain("&lt;script&gt;")
    expect(html).not.toContain("<script>")
  })

  test("marks SObject constructor field names as fields, not classes", () => {
    const html = highlightApex("Account account = new Account(Name = name);")

    expect(html).toContain('<span class="token sobject-field">Name</span>')
    expect(html).not.toContain('<span class="token class-name">Name</span>')
  })
})
