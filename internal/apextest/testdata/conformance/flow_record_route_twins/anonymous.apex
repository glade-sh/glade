Map<String,Object> observations=new Map<String,Object>();
{ Savepoint point=Database.setSavepoint(); observations.put('interview_direct',BoundaryFlowProbe.observe('interview_direct')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('interview_subflow',BoundaryFlowProbe.observe('interview_subflow')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('invocable_direct',BoundaryFlowProbe.observe('invocable_direct')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('invocable_subflow',BoundaryFlowProbe.observe('invocable_subflow')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('before_create',BoundaryFlowProbe.observe('before_create')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('before_update',BoundaryFlowProbe.observe('before_update')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('after_direct_create',BoundaryFlowProbe.observe('after_direct_create')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('after_direct_update',BoundaryFlowProbe.observe('after_direct_update')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('after_subflow_create',BoundaryFlowProbe.observe('after_subflow_create')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('after_subflow_update',BoundaryFlowProbe.observe('after_subflow_update')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('delete_direct',BoundaryFlowProbe.observe('delete_direct')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('delete_subflow',BoundaryFlowProbe.observe('delete_subflow')); Database.rollback(point); }
{ Savepoint point=Database.setSavepoint(); observations.put('scheduled_dml',BoundaryFlowProbe.observe('scheduled_dml')); Database.rollback(point); }
System.assert(false,'OBS|'+JSON.serialize(observations));
