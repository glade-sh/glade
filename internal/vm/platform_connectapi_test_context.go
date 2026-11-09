package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

// ConnectApi service calls in a data-siloed Apex test (SeeAllData=false).
//
// Native rows (fresh scratch org, APIs 62.0 and 67.0, @IsTest class route; see
// internal/apextest/testdata/conformance/connectapi_services.json):
//   - a service method with a setTest<Method> counterpart returns the result the
//     test registered for the same arguments, the very same object;
//   - with no registered result it fails the test with an uncatchable
//     System.AssertException naming the setTest method to call;
//   - every other service method throws System.UnsupportedOperationException:
//     "ConnectApi methods are not supported in data siloed tests. Please use
//     @IsTest(SeeAllData=true)." after the method's own argument checks.
//
// Glade never implements the real services. Outside data-siloed tests the
// existing local dispatch applies.
const connectAPIDataSiloedMessage = "ConnectApi methods are not supported in data siloed tests. Please use @IsTest(SeeAllData=true)."

func (vm *VM) connectAPIDataSiloedTest() bool {
	return vm.testContext != nil && vm.testContext.SeeAllDataSet && !vm.testContext.SeeAllData
}

// connectAPIServiceClass reports a platform ConnectApi class with static
// service methods; project classes that shadow the namespace never match.
func (vm *VM) connectAPIServiceClass(className string) bool {
	if !hasTypePrefixFold(className, "ConnectApi") {
		return false
	}
	generated, ok := generatedPlatformTypes()[strings.ToLower(className)]
	return ok && generated.Kind != apexast.DeclarationEnum
}

func (vm *VM) callConnectAPIServiceInSiloedTest(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok || !vm.connectAPIServiceClass(className) {
		return Null, false, nil
	}
	if connectAPITestSetterName(methodName) {
		return vm.callConnectAPITestSetter(className, methodName, args)
	}
	if !vm.connectAPIDataSiloedTest() {
		return Null, false, nil
	}
	method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
	if !ok {
		method, ok = vm.generatedPlatformStaticMethodByNameArity(className, methodName, len(args))
	}
	if !ok {
		return Null, false, nil
	}
	if err := connectAPIArgumentError(className, method, args); err != nil {
		return Null, true, err
	}
	if vm.connectAPIHasTestSetter(className, methodName) {
		if vm.testContext.ConnectAPIFixtures != nil {
			if value, found := vm.testContext.ConnectAPIFixtures[connectAPITestFixtureKey(className, methodName, args)]; found {
				return value, true, nil
			}
		}
		return Null, true, vm.assertError("Assertion Failed: " + connectAPIMissingTestResultMessage(className, method))
	}
	return Null, true, newExceptionError("System.UnsupportedOperationException", connectAPIDataSiloedMessage)
}

// callConnectAPITestSetter registers a setTest result inside any Apex test.
// Outside a test the native call throws System.UnsupportedOperationException
// "<Class>.<setTestMethod>(<params>) may only be called in tests" (rows
// call.*.setTest*.null, anonymous route).
func (vm *VM) callConnectAPITestSetter(className, methodName string, args []Value) (Value, bool, error) {
	setter, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
	if !ok {
		setter, ok = vm.generatedPlatformStaticMethodByNameArity(className, methodName, len(args))
	}
	if !ok || len(args) == 0 {
		return Null, false, nil
	}
	if vm.testContext == nil {
		return Null, true, newExceptionError("System.UnsupportedOperationException",
			connectAPILocalClassName(className)+"."+apexMethodMemberName(setter.Name)+"("+connectAPIParamList(setter.Params)+") may only be called in tests")
	}
	if vm.testContext.ConnectAPIFixtures == nil {
		vm.testContext.ConnectAPIFixtures = make(map[string]Value)
	}
	target := connectAPITestSetterTarget(methodName)
	vm.testContext.ConnectAPIFixtures[connectAPITestFixtureKey(className, target, args[:len(args)-1])] = args[len(args)-1]
	return Null, true, nil
}

func connectAPILocalClassName(className string) string {
	if dot := strings.IndexByte(className, '.'); dot >= 0 {
		return className[dot+1:]
	}
	return className
}

func connectAPIParamList(params []Param) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, connectAPIDisplayTypeName(param.Type)+" "+param.Name)
	}
	return strings.Join(parts, ", ")
}

func (vm *VM) connectAPIHasTestSetter(className, methodName string) bool {
	methods := generatedPlatformMethods()[strings.ToLower(className)]
	setters := methods["settest"+strings.ToLower(methodName)]
	for _, setter := range setters {
		if setter.IsStatic {
			return true
		}
	}
	return false
}

// connectAPIMissingTestResultMessage is the native text, for example
// "No matching test result found for ChatterFeeds.searchFeedElements(String
// communityId, String q). Before calling this, call
// ChatterFeeds.setTestSearchFeedElements(String communityId, String q,
// ConnectApi.FeedElementPage result) to set the expected test result."
func connectAPIMissingTestResultMessage(className string, method Method) string {
	local := connectAPILocalClassName(className)
	params := connectAPIParamList(method.Params)
	name := apexMethodMemberName(method.Name)
	setter := "setTest" + strings.ToUpper(name[:1]) + name[1:]
	setterParams := connectAPIDisplayTypeName(method.ReturnType) + " result"
	if params != "" {
		setterParams = params + ", " + setterParams
	}
	return "No matching test result found for " + local + "." + name + "(" + params + "). " +
		"Before calling this, call " + local + "." + setter + "(" + setterParams + ") to set the expected test result."
}

func connectAPIDisplayTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if hasTypePrefixFold(typeName, "System") {
		return typeName[len("System."):]
	}
	return typeName
}

// connectAPIArgumentRule is one captured argument check that runs before the
// data-siloed boundary (and before a setTest lookup). Native rows
// call.<Class>.<method>.null and .validid at APIs 62.0 and 67.0:
//   - required: a null argument throws ConnectApiException "Parameter required: <name>";
//   - idFormat: a value that is not a 15- or 18-character id throws
//     "Illegal value for parameter: '<name>': <value>";
//   - idPrefix: an id with another key prefix throws the same message.
//
// Methods without a captured rule reach the boundary with no argument check.
type connectAPIArgumentRule struct {
	index    int
	param    string
	required bool
	idFormat bool
	idPrefix string
	// rejectTested rejects a well-formed id whose key prefix was tested and is
	// not rejectExempt. A native scan accepted none of the fake ids, because
	// the id must name an existing record; the prefix of a real one is exempt.
	rejectTested bool
	rejectExempt string
	// acceptFold names a literal accepted in any case; the Knowledge
	// communityId takes "internal" (capi3_knowledge_community/internal.tsv).
	acceptFold string
}

var connectAPIArgumentRules = map[string][]connectAPIArgumentRule{
	"connectapi.chatterfeeds.deletecomment":                                     {{index: 1, param: "commentId", required: true, idFormat: true, idPrefix: "0D7"}},
	"connectapi.chatterfeeds.deletefeedelement":                                 {{index: 1, param: "feedElementId", required: true, idFormat: true}},
	"connectapi.chatterfeeds.getcommentsforfeedelement":                         {{index: 1, param: "feedElementId", required: true, idFormat: true}},
	"connectapi.chatterfeeds.getfeedelement":                                    {{index: 1, param: "feedElementId", required: true, idFormat: true}},
	"connectapi.chatterfeeds.getfeedelementbatch":                               {{index: 1, param: "feedElementIds", required: true}},
	"connectapi.chatterfeeds.getfeedelementsfromfeed":                           {{index: 1, param: "feedType", required: true}},
	"connectapi.chatterfeeds.iscommenteditablebyme":                             {{index: 1, param: "commentId", required: true, idFormat: true, idPrefix: "0D7"}},
	"connectapi.chatterfeeds.isfeedelementeditablebyme":                         {{index: 1, param: "feedElementId", required: true, idFormat: true}},
	"connectapi.chatterfeeds.postcommenttofeedelement":                          {{index: 1, param: "feedElementId", required: true, idFormat: true}},
	"connectapi.chatterfeeds.updatecomment":                                     {{index: 1, param: "commentId", required: true, idFormat: true, idPrefix: "0D7"}},
	"connectapi.chatterfeeds.updatefeedelement":                                 {{index: 1, param: "feedElementId", required: true, idFormat: true}},
	"connectapi.einsteinllm.generatemessagesforprompttemplate":                  {{index: 0, param: "promptTemplateDevName", required: true}},
	"connectapi.chatterusers.getfollowings":                                     {{index: 1, param: "userId", required: true}},
	"connectapi.namedcredentials.createexternalauthidentityprovidercredentials": {{index: 0, param: "fullName", required: true}},
	"connectapi.namedcredentials.deleteexternalauthidentityprovider":            {{index: 0, param: "fullName", required: true}},
	"connectapi.namedcredentials.deleteexternalcredential":                      {{index: 0, param: "developerName", required: true}},
	"connectapi.namedcredentials.deletenamedcredential":                         {{index: 0, param: "developerName", required: true}},
	"connectapi.namedcredentials.getexternalauthidentityprovider":               {{index: 0, param: "fullName", required: true}},
	"connectapi.namedcredentials.getexternalauthidentityprovidercredentials":    {{index: 0, param: "fullName", required: true}},
	"connectapi.namedcredentials.getexternalcredential":                         {{index: 0, param: "developerName", required: true}},
	"connectapi.namedcredentials.getnamedcredential":                            {{index: 0, param: "developerName", required: true}},
	"connectapi.namedcredentials.updateexternalauthidentityprovidercredentials": {{index: 0, param: "fullName", required: true}},
	"connectapi.namedcredentials.updateexternalcredential":                      {{index: 0, param: "developerName", required: true}},
	"connectapi.namedcredentials.updatenamedcredential":                         {{index: 0, param: "developerName", required: true}},
	"connectapi.nextbestaction.executestrategy":                                 {{index: 0, param: "strategyName", required: true}},
	"connectapi.questionandanswers.updatequestionandanswers":                    {{index: 1, param: "feedElementId", required: true, idFormat: true}},
	"connectapi.recordui.getpicklistvaluesbyrecordtype": {
		{index: 0, param: "objectApiName", required: true},
		{index: 1, param: "recordTypeId", idFormat: true, idPrefix: "012"},
	},
	"connectapi.knowledge.gettopviewedarticlesfortopic": {
		{index: 0, param: "communityId", idFormat: true, rejectTested: true, rejectExempt: "0DB", acceptFold: "internal"},
		{index: 1, param: "topicId", required: true, idFormat: true, idPrefix: "0TO"},
	},
	"connectapi.knowledge.gettrendingarticles": {
		{index: 0, param: "communityId", idFormat: true, rejectTested: true, rejectExempt: "0DB", acceptFold: "internal"},
	},
	"connectapi.knowledge.gettrendingarticlesfortopic": {
		{index: 0, param: "communityId", idFormat: true, rejectTested: true, rejectExempt: "0DB", acceptFold: "internal"},
		{index: 1, param: "topicId", required: true, idFormat: true, idPrefix: "0TO"},
	},
	"connectapi.topics.assigntopic":       {{index: 1, param: "recordId", required: true}},
	"connectapi.userprofiles.deletephoto": {{index: 1, param: "userId", required: true}},
	"connectapi.userprofiles.getphoto":    {{index: 1, param: "userId", required: true}},
	"connectapi.userprofiles.setphoto":    {{index: 1, param: "userId", required: true}},
}

func connectAPIArgumentError(className string, method Method, args []Value) error {
	paramTypes := make([]string, len(method.Params))
	for i, param := range method.Params {
		paramTypes[i] = strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(param.Type), "system.", ""), " ", "")
	}
	key := strings.ReplaceAll(strings.ToLower(className)+"."+strings.ToLower(apexMethodMemberName(method.Name))+"("+strings.Join(paramTypes, ",")+")", "system.", "")
	// Favorites validate the subject before the favorite ID. Captured rows
	// accept "me" and the User prefix, and reject the other tested prefixes.
	if rule, ok := connectAPIUserIDRule(key); ok && strings.EqualFold(className, "ConnectApi.ChatterFavorites") && strings.EqualFold(rule.param, "favoriteId") && len(args) > 1 {
		subject := args[1]
		if subject.Kind == ValueString && subject.Text != "me" {
			invalid := !connectAPIWellFormedID(subject.Text)
			if !invalid {
				prefix := "|" + subject.Text[:3] + "|"
				invalid = strings.Contains(connectAPITestedIDPrefixes, prefix) && subject.Text[:3] != "005"
			}
			if invalid {
				return newExceptionError("ConnectApi.ConnectApiException", "Illegal value for parameter: 'subjectId': "+subject.Text)
			}
		}
	}
	if rule, ok := connectAPIUserIDRule(key); ok && rule.index < len(args) {
		arg := args[rule.index]
		if arg.Kind == ValueString {
			invalid := false
			if strings.EqualFold(rule.param, "keyPrefix") {
				invalid = len(arg.Text) != 3
			} else if key == "connectapi.topics.getrelatedtopics(string,string)" && arg.Text == "me" {
				// The captured topic lookup accepts this sentinel.
			} else if !connectAPIWellFormedID(arg.Text) {
				invalid = true
			} else if accepted, captured := connectAPIAcceptedPrefixes(key); captured && accepted != "*" {
				prefix := "|" + arg.Text[:3] + "|"
				invalid = strings.Contains(connectAPITestedIDPrefixes, prefix) && !strings.Contains(accepted, prefix)
			}
			if invalid {
				return newExceptionError("ConnectApi.ConnectApiException", "Illegal value for parameter: '"+rule.param+"': "+arg.Text)
			}
		}
	}
	// Filter-feed checks keyPrefix first, then the separately captured subjectId.
	if rule, ok := connectAPIUserIDRule(key); ok && strings.EqualFold(rule.param, "keyPrefix") && len(args) > 1 {
		subject := args[1]
		if subject.Kind == ValueString && subject.Text != "me" && ((len(subject.Text) != 15 && len(subject.Text) != 18) || strings.Trim(subject.Text, "0") == "") {
			return newExceptionError("ConnectApi.ConnectApiException", "Illegal value for parameter: 'subjectId': "+subject.Text)
		}
	}
	rules := connectAPIArgumentRules[strings.ToLower(className)+"."+strings.ToLower(apexMethodMemberName(method.Name))]
	for _, rule := range rules {
		// Generated overloads may carry positional names, so the captured
		// position identifies the argument and the captured name is reported.
		index := rule.index
		for i, param := range method.Params {
			if strings.EqualFold(param.Name, rule.param) {
				index = i
				break
			}
		}
		if index >= len(args) {
			continue
		}
		arg := args[index]
		if arg.Kind == ValueNull {
			if rule.required {
				return newExceptionError("ConnectApi.ConnectApiException", "Parameter required: "+rule.param)
			}
			continue
		}
		if !rule.idFormat || arg.Kind != ValueString {
			continue
		}
		text := arg.Text
		if rule.acceptFold != "" && strings.EqualFold(text, rule.acceptFold) {
			continue
		}
		rejected := rule.rejectTested && connectAPIWellFormedID(text) && text[:3] != rule.rejectExempt && strings.Contains(connectAPITestedIDPrefixes, "|"+text[:3]+"|")
		if rejected || !connectAPIWellFormedID(text) || (rule.idPrefix != "" && !strings.HasPrefix(text, rule.idPrefix)) {
			return newExceptionError("ConnectApi.ConnectApiException", "Illegal value for parameter: '"+rule.param+"': "+text)
		}
	}
	return nil
}

func connectAPIWellFormedID(text string) bool {
	if len(text) != 15 && len(text) != 18 {
		return false
	}
	for _, r := range text {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}
