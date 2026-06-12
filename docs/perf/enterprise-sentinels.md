# Enterprise Performance Sentinels

## Required Gates

- `apex-recipes`: small correctness and timing smoke.
- `sf-cred`: alias/static tracking and setup-heavy tests.
- `nu`: 11k+ enterprise suite and order/payment heavy classes.
- `nams`: imported membership and expression-heavy classes.
- `nutpl`: fast fflib/package sentinel.

## Rules

- Use `--parallel 4` for long local-test runs.
- Use `--class` and `--method`, not `--filter`.
- Prefer exact slow methods from saved JSON before full-suite runs.
- Full `nu` and `nams` runs require explicit approval unless a release gate needs them.

## Current Saved Artifacts

- `test-results/recipes.json`
- `test-results/sf-cred.json`
- `test-results/nu.json`
- `test-results/nams.json`
- `test-results/nutpl.json`

## Merge Gates

- Focused package tests pass.
- `scripts/perf/glade-baseline.sh` shows no cold-start regression.
- At least one enterprise sentinel target shows measured gain.
- No sentinel target changes pass/fail behavior.
- `git diff --check` passes.

## Slow Targets

Columns: class, tests, total ms, max method ms, slow method.

### `nu`

| Class | Tests | Total ms | Max ms | Slow Method |
| --- | ---: | ---: | ---: | --- |
| `TestOrderPaymentController` | 137 | 355982 | 6971 | `savePayment_forRefund_noCSCIsSet_CSCRequiredOnEntity_expectNoPageMessage` |
| `CartSubmitterTest` | 163 | 327420 | 8410 | `submit_refundCart_expectPaymentAndPaymentLinesLinkedToOrder` |
| `CartServiceTest` | 63 | 235628 | 8264 | `submit_couponStillValid_expectCouponHistoryRecordCreated` |
| `TestCartSubmitter` | 79 | 230015 | 12081 | `deletePaymentFromCartPayment_interEntityRefundMarkedForDeletion_expectInterEntityTransactionsDeleted` |
| `TestOrderController` | 110 | 185248 | 14776 | `cancelEventRegistrationOrderTest` |
| `TestOrderPurchaseEvent` | 36 | 180461 | 13741 | `cancelAdditionalBadgeTest` |
| `OrderServiceTest` | 52 | 167423 | 7792 | `convertOrderToCart_paymentLinesForSamePayment_expectSingleCartPaymentWithSummedAmount` |
| `TransactionGenerator2Test` | 66 | 140812 | 12123 | `generate_deletePaymentFromAlreadyCancelledItem_ARIsBalanced` |
| `OrderProcessorTest` | 41 | 137951 | 6119 | `process_withRequestForMultiplePaymentsForMultipleOrders_expectOrdersProcessedAndPaid` |
| `AffiliationTriggerHandlers2Test` | 41 | 137506 | 26208 | `bulkInsertOfPrimaryContactAffiliations` |

### `nams`

| Class | Tests | Total ms | Max ms | Slow Method |
| --- | ---: | ---: | ---: | --- |
| `MembershipBillingSuite` | 33 | 549392 | 243279 | `simplestRenewal_bulk_200importedMemberships` |
| `expr_CollectionFunctionsTest` | 151 | 152625 | 3207 | `expandFunctionSupportsReferencingChilddata` |
| `RTest` | 165 | 68254 | 1197 | `dbDeleteTest` |
| `CommerceCheckoutUiTest` | 52 | 59171 | 3484 | `processesCustomerInformation` |
| `expr_QueryTest` | 85 | 56863 | 1414 | `canCombineQueriesWithLetToUseCustomVariables` |
| `expr_StringFunctionsTest` | 70 | 48781 | 852 | `substituteReplacesAllOccurrencesOfTheFirstArgumentWithTheSecondArgument` |
| `ProFormaOrderServiceTest` | 30 | 48343 | 4125 | `populatesErrorsWhileValidatingOrder` |
| `fflib_MatcherDefinitionsTest` | 78 | 43625 | 869 | `whenConstructingCombinedWithNullConnectiveExpressionShouldThrowException` |
| `SubscriptionBillingSuite` | 3 | 38661 | 22874 | `simplestRenewal_renewAndProcess_andRenewAgain` |
| `NimbleAmsPricingFunctionsTest` | 37 | 33649 | 2096 | `hasAttendedEvent_customerHasActiveRegistrationForEvent_expectTrue` |

### `sf-cred`

| Class | Tests | Total ms | Max ms | Slow Method |
| --- | ---: | ---: | ---: | --- |
| `tst_dataMapper` | 4 | 72106 | 71815 | `tst_dataMapper` |
| `MappedProviderTriggerHandlerTest` | 34 | 58108 | 8312 | `testAfterDelete_MixedScenarioBulk_EachProviderHandledCorrectly` |
| `CredentialingWorkflowTriggerHandlerTest` | 29 | 41751 | 12315 | `deleteChildCredentialingRequestEvents_deleteEventsWithChildren` |
| `ProviderServiceTest` | 47 | 40412 | 4012 | `testDeleteLogicallyRelatedProviderChildRecords` |
| `tst_webHook` | 32 | 36168 | 3865 | `tst_datasetScanCompletedAMA_Training` |
| `FacilityInfoSpecialtiesControllerTest` | 25 | 33468 | 3099 | `saveFacilitySpecialtyAssociationsSuccessWhenAppliesToChangesFromSelfAndAllChildrenToCustom_FacilitySpecialtyIdsNotEmptyAndThereAreExistingAssociations` |
| `PurgeDatasetScansBatchTest` | 11 | 30282 | 14754 | `testWhenScanHasResults_AllSE` |
| `PostInstallScriptTest` | 31 | 29106 | 1628 | `whenSetSimplifiedLicenseMappingCommitsWork` |
| `fflib_MatcherDefinitionsTest` | 124 | 27256 | 387 | `whenSObjectWithMatchesShouldReturnCorrectResults` |
| `SOQL_Test` | 238 | 23462 | 354 | `toValueOf` |

### `nutpl`

| Class | Tests | Total ms | Max ms | Slow Method |
| --- | ---: | ---: | ---: | --- |
| `fflib_MatcherDefinitionsTest` | 78 | 12909 | 1735 | `whenAnyFieldSetMatchesShouldReturnCorrectResults` |
| `fflib_MatchTest` | 93 | 6886 | 1467 | `fieldSetEquivalentToRegistersCorrectMatcherType` |
| `fflib_ApexMocksTest` | 72 | 6605 | 150 | `thatStubbingMultipleMethodsCanBeChainedFirstValueThenException` |
| `fflib_InOrderTest` | 62 | 6346 | 162 | `thatWithOldNotationThrowsExceptionIfCalledLessTimesThanExpected` |
| `fflib_QueryFactoryTest` | 35 | 5516 | 656 | `addChildQueriesWithChildRelationship_success` |
| `fflib_AnyOrderTest` | 49 | 4706 | 145 | `thatVerifiesAtLeastOnceWithMatchers` |
| `TemplateFactoryTest` | 16 | 2725 | 656 | `getTemplateOutput_oneHundredAccounts_expectMergeFeldsReplaced` |
| `fflib_ArgumentCaptorTest` | 25 | 2526 | 126 | `thatDoesNotCaptureAnythingWhenCaptorIsWrappedInAMatcher` |
| `TemplateExprEvaluationContextTest` | 30 | 2512 | 127 | `lessThanCondition_expectedFalse` |
| `SubtemplateWrapperTest` | 22 | 2107 | 159 | `constructor_validMarkup_expectPropertiesSet` |

### `apex-recipes`

| Class | Tests | Total ms | Max ms | Slow Method |
| --- | ---: | ---: | ---: | --- |
| `AccountTriggerHandler_Tests` | 9 | 4783 | 1030 | `afterUpdateTestPositive` |
| `DMLRecipes_Tests` | 40 | 3221 | 226 | `testInsertInSystemModePositive` |
| `SOQLRecipes_Tests` | 20 | 3101 | 962 | `testCountOfLargeDataVolumesPositive` |
| `CustomRestEndpointRecipes_Tests` | 22 | 1803 | 237 | `httpCalloutGetRecordsToReturnPositive` |
| `StripInaccessibleRecipes_Tests` | 10 | 1672 | 433 | `testStripInaccessibleFromSubQueryMinAccessWithpermsetPositive` |
| `Safely_Tests` | 16 | 1513 | 195 | `testDoInsertMethodsNoThrowPositive` |
| `BatchApexRecipes_Tests` | 2 | 1316 | 690 | `batchApexRecipesTestPositive200Scope` |
| `MetadataCatalogRecipes_Tests` | 2 | 1286 | 1270 | `testFindAllFormulaFieldsPositive` |
| `QueueableWithCalloutRecipes_Tests` | 2 | 1150 | 609 | `testQueueableWithCalloutRecipesPositive` |
| `CalloutRecipes_Tests` | 14 | 960 | 177 | `httpGetCalloutToSecondSalesforceOrgPositive` |
