Fresh 30-day developer scratch org; Salesforce CLI; 2026-11-09.
Deployed the synthetic project. Anonymous Apex ran versioned classes at API 62.0 and 67.0. Each output row records the observed default value and formula. The invalid field was separately deployed as a dry run at each API and rejected; its source is reject.field-meta.xml.
Scratch org deletion succeeded; a subsequent alias lookup returned NamedOrgNotFoundError.
A second fresh 30-day developer scratch org deployed the project and ran TextDefaultMatrixTest62.observe and TextDefaultMatrixTest67.observe; both passed with assertions for each recorded value and formula.
The second scratch org was deleted successfully; a subsequent alias lookup returned NamedOrgNotFoundError.
