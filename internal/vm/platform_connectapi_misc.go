package vm

import "fmt"

func (vm *VM) connectAPITopicsGetTopicSuggestions(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return Null, fmt.Errorf("ConnectApi.Topics.getTopicSuggestions expects 2-3 arguments")
	}
	result := Object("ConnectApi.TopicSuggestionPage")
	result.Fields["currentPageToken"] = String("0")
	result.Fields["nextPageToken"] = Null
	suggestions := typedList("List<ConnectApi.TopicSuggestion>")
	result.Fields["topicSuggestions"] = suggestions
	return result, nil
}

func (vm *VM) connectAPIWaveExecuteQuery(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return Null, fmt.Errorf("ConnectApi.Wave.executeQuery expects 1-2 arguments")
	}
	result := Object("ConnectApi.LiteralJson")
	result.Fields["json"] = String("{\"results\":[]}")
	result.Fields["url"] = String("/services/data/vXX.X/wave/query")
	return result, nil
}
