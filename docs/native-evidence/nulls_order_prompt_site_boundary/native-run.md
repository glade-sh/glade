# Native capture: inactive site and prompt DML boundary

A fresh 30-day Developer scratch org was created from a campaign Dev Hub. Synthetic inactive Visualforce site metadata and a synthetic landing page were deployed. At API 62 and 67, the Apex test saw one Site row, but system-mode DML rejected both synthetic Prompt parents and a PromptVersion. The test only records setup and rejection; it does not establish NULLS ordering or rule out metadata creation. No site was activated or accessed. The scratch org was deleted and ScratchOrgInfo status verified as Deleted.
