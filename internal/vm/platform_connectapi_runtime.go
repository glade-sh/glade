package vm

import (
	"fmt"
	"strings"
)

func (vm *VM) connectApiNBAExecuteStrategy(args []Value) (Value, error) {
	if len(args) != 4 {
		return Null, fmt.Errorf("ConnectApi.NextBestAction.executeStrategy expects 4 arguments")
	}
	result := Object("ConnectApi.NBARecommendations")
	result.Fields["executionId"] = String("glade-nba-execution-1")
	result.Fields["onBehalfOfId"] = args[2]
	result.Fields["trace"] = typedList("List<Object>")
	recommendations := typedList("List<ConnectApi.NBARecommendation>")
	for i := 0; i < connectApiRecommendationLimit(args[1]); i++ {
		recommendations.List = append(recommendations.List, vm.connectApiNBARecommendation(i+1, args[0], args[2]))
	}
	result.Fields["recommendations"] = recommendations
	return result, nil
}

func connectApiRecommendationLimit(value Value) int {
	limit := 1
	if value.Kind == ValueInt {
		limit = int(value.Int)
	}
	if limit < 0 {
		return 0
	}
	if limit > 10 {
		return 10
	}
	return limit
}

func (vm *VM) connectApiNBARecommendation(index int, strategy, contextRecord Value) Value {
	suffix := fmt.Sprintf("%d", index)
	target := Object("ConnectApi.NBANativeRecommendation")
	target.Fields["id"] = String("0nb00000000000" + suffix)
	target.Fields["name"] = String("Local Recommendation " + suffix)
	target.Fields["url"] = String("/lightning/r/Recommendation/" + suffix + "/view")

	parameter := Object("ConnectApi.NBAActionParameter")
	parameter.Fields["name"] = String("recordId")
	parameter.Fields["type"] = String("String")
	parameter.Fields["value"] = contextRecord

	action := Object("ConnectApi.NBAFlowAction")
	action.Fields["id"] = String("30000000000000" + suffix)
	action.Fields["name"] = String("LocalRecommendationFlow")
	action.Fields["flowLabel"] = String("Local Recommendation Flow")
	action.Fields["flowType"] = vm.connectApiEnumValue("ConnectApi.NBAFlowType", "AutoLaunchedFlow")
	parameters := typedList("List<ConnectApi.NBAActionParameter>")
	parameters.List = append(parameters.List, parameter)
	action.Fields["parameters"] = parameters

	rec := Object("ConnectApi.NBARecommendation")
	rec.Fields["acceptanceLabel"] = String("Accept")
	rec.Fields["aiModel"] = Null
	rec.Fields["description"] = String("Local Next Best Action recommendation")
	rec.Fields["externalId"] = String("glade-nba-" + suffix)
	rec.Fields["imageUrl"] = Null
	rec.Fields["recommendationMode"] = Null
	rec.Fields["recommendationScore"] = Int(100)
	rec.Fields["rejectionLabel"] = String("Reject")
	rec.Fields["target"] = target
	rec.Fields["targetAction"] = action
	if strategy.Kind == ValueString && strings.TrimSpace(strategy.Text) != "" {
		rec.Fields["strategyName"] = strategy
	}
	return rec
}

func (vm *VM) connectApiNBASetRecommendationReaction(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("ConnectApi.NextBestAction.setRecommendationReaction expects 1 argument")
	}
	input := args[0]
	result := Object("ConnectApi.RecommendationReaction")
	result.Fields["id"] = String("0rr000000000001")
	result.Fields["createdDate"] = String("2026-06-07T00:00:00Z")
	result.Fields["createdBy"] = Null
	result.Fields["url"] = String("/services/data/vXX.X/connect/recommendation-reactions/0rr000000000001")
	copyObjectField(result, input, "reactionType")
	copyObjectField(result, input, "externalId")
	copyObjectField(result, input, "recommendationMode")
	copyObjectField(result, input, "recommendationScore")
	copyObjectField(result, input, "aiModel")
	copyObjectField(result, input, "strategyName")
	copyObjectFieldAs(result, input, "contextRecordId", "contextRecord")
	copyObjectFieldAs(result, input, "onBehalfOfId", "onBehalfOf")
	copyObjectFieldAs(result, input, "targetId", "targetRecord")
	copyObjectFieldAs(result, input, "targetActionName", "targetAction")
	return result, nil
}

func (vm *VM) connectApiOrchGetInstanceCollection(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.Orchestrator.getOrchestrationInstanceCollection expects 1 or 2 arguments")
	}
	result := Object("ConnectApi.OrchestrationInstanceCollection")
	result.Fields["instances"] = typedList("List<ConnectApi.OrchestrationInstance>")
	return result, nil
}

func (vm *VM) connectApiOrchPublishEvent(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("ConnectApi.Orchestrator.publishOrchestrationEvent expects 1 argument")
	}
	info := args[0]
	result := Object("ConnectApi.OrchestrationEvent")
	result.Fields["isSuccess"] = Bool(true)
	result.Fields["workStatus"] = vm.connectApiObjectFieldOrEnum(info, "workStatus", "ConnectApi.OrchestrationWorkStatus", "FlowCompleted")
	copyObjectField(result, info, "orchestrationInstanceId")
	copyObjectField(result, info, "stageStepInstanceId")
	copyObjectField(result, info, "workAssignmentId")
	return result, nil
}

func (vm *VM) connectApiObjectFieldOrEnum(value Value, fieldName, typeName, enumName string) Value {
	if value.Kind == ValueObject {
		if existing, ok := value.Fields[fieldName]; ok && existing.Kind != ValueNull {
			return existing
		}
	}
	return vm.connectApiEnumValue(typeName, enumName)
}

func (vm *VM) connectApiEnumValue(typeName, name string) Value {
	if value, ok := vm.generatedPlatformStaticFieldValue(typeName, name); ok {
		return value
	}
	return Value{Kind: ValueObject, Type: typeName, Text: name, Fields: map[string]Value{"ordinal": Int(-1)}}
}

func copyObjectField(target Value, source Value, fieldName string) {
	copyObjectFieldAs(target, source, fieldName, fieldName)
}

func copyObjectFieldAs(target Value, source Value, sourceField, targetField string) {
	if target.Fields == nil || source.Kind != ValueObject {
		return
	}
	if value, ok := source.Fields[sourceField]; ok {
		target.Fields[targetField] = value
	}
}
