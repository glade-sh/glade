# Lazy getter through a helper

Scratch org, Salesforce CLI 2.157.8, 2026-11-10, APIs 62.0 and 67.0, IsTest route.

A property getter that returns `this.readBacking()`, where the helper reads
the same property, recurses to `Maximum stack depth reached: 1001`. This holds
with a lazy-initialising null check and assignment in the getter before the
helper call (`full`), with only the check (`guardOnly`), with only the
assignment (`assignOnly`), and with neither (`plain`).
