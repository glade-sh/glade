Fresh owned scratch org (Developer edition, campaign Dev Hub), 2026-11-08; deleted afterwards (ScratchOrgInfo
Status=Deleted). The earlier cases were recaptured here with identical rows. Deployed project/force-app at source API 67.0: every synthetic process deployed and activated
(processType Workflow, Flow API 49.0, except Fern_Met62_Process at 62.0 and Fern_Met67_Process at 67.0, which also
carry <areMetricsLoggedToDataCloud>false). Permission set Fern_Access was assigned to the running admin user.
Routes: synchronous named tests FernCases62Test (class API 62.0) and FernCases67Test (67.0) with sf apex run test
--api-version 62.0 and 67.0; anonymous Apex calling FernCases62.<case>() or FernCases67.<case>() at sf apex run
--api-version 62.0 and 67.0 (the row is read from a USER_DEBUG line; the script then fails its assertion to roll back).
Rows are the text after "Actual: " (test route) or the USER_DEBUG text (anonymous). A row lists each step of the case:
"label: <events> => <rows read back>". An event is Object.Phase+Trigger.size(first record) for a trigger, with
<Trigger.old Note__c/Score__c> on update events, or Tag+size(first record) for an invocable action. FernLog.failure replaces the "Error ID:" tail and 15/18 character ids.
Rows are identical at 62.0 and 67.0 on both routes, apart from the api= prefix, with two route differences: an Apex
action failure (thr, thrbulk) ends with "Error ID: X" on the anonymous route and has none on the test route; and the test
route cuts a long assertion message (fan, mergep end early there), so the anonymous row is the full text.
Shapes:
- upd: record update with an implicit Id filter and formula, reference, boolean, date and empty values.
- two, twoorder: two criteria nodes, no evaluate-next; custom logic "(1 OR 2) AND 3"; mixed cohorts.
- evalnext, evalbulk: evaluate-next wired by the action's connector; a later criterion reads the record as updated.
- grp: Apex, record update, Apex action group, single and bulk.
- asg: myVariable_waitStartTimeAssignment ahead of the first decision.
- chg: isChangedDecision outcome used as a condition; a previously-met decision.
- met: areMetricsLoggedToDataCloud, Flow API 62.0 and 67.0.
- rel: parent update, null parent, criteria-filtered update, child update from the parent's process.
- ord, ordbulk: order against triggers, a workflow rule, before-save and after-save flows.
- seq, bmp, rec: nested save of a record in the running process, from a record update and from an Apex action.
- sib: a record update with a non-Id filter reaches another record of the object, in and out of the cohort.
- created, pri: onCreateOnly; ISNEW and PRIORVALUE criteria; a myVariable_old value.
- err: a process record update fails validation, all-or-none and partial save. The message names the flow label.
- flowact: an autolaunched flow action, single and bulk.
- thr: an Apex action throws, all-or-none and partial save.
- cgl: custom logic over an isChanged outcome; field and formula right operands.
- tpl: isTemplate and areMetricsLoggedToDataCloud at Flow API 65.0.
- eml: an email alert ahead of a record update; Limits.getEmailInvocations() stays 0.
- fan, fanmulti: part of a bulk record update fails. The element saves as a partial save: survivors fire their triggers
  again. The process then stops for every record, after-save flows still run for the survivors, and failed rows carry the
  details joined by "---".
- thrbulk: an Apex action fails for a cohort: one message, on each of its rows.
- mergep, sibmerge: interviews reaching one target. Same values save once; different values save in a later DML.
- flow2, flow2fail: a flow action whose flow has two record updates; each saves the cohort; one fails validation.
- asn, asu, asubulk: nested saves and record updates from the after-save flow phase.
- ordsaves: after-save Apex action batch on bulk update, partial insert and upsert.
- errups: a partial upsert retries its surviving rows.
Not captured, still GLADEAUTO002: several active processes on one object, a flow action on a flow with other steps,
formula functions the evaluator does not read, waits (scheduled actions), Chatter and other action types, record creates,
RecursiveCountVariable.
API 68 not run (no 62/67 difference).
