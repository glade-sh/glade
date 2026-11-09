import assert from 'node:assert/strict';
import {executeMutation} from '../../../../lwcruntime/src/lightning/graphql.mjs';
const result=await executeMutation({query:{kind:'Document'}});
assert.deepEqual(result,{errors:[{errorType:'localBoundary',message:'GraphQL mutation execution is not supported locally'}]});
