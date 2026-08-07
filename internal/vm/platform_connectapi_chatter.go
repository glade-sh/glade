package vm

import (
	"fmt"
	"net/url"
)

func (vm *VM) connectAPIChatterUsersGetFollowings(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 5 {
		return Null, fmt.Errorf("ConnectApi.ChatterUsers.getFollowings expects 2-5 arguments")
	}
	if vm.testContext != nil && vm.testContext.SeeAllDataSet && !vm.testContext.SeeAllData {
		return Null, newExceptionError("UnsupportedOperationException", "ConnectApi.ChatterUsers.getFollowings requires SeeAllData=true in local tests")
	}
	pageURL := "/services/data/vXX.X/chatter/users/" + scalarText(args[1]) + "/following"
	query := url.Values{}
	if len(args) >= 3 {
		if args[2].Kind == ValueString {
			query.Set("filterType", args[2].Text)
			if len(args) >= 4 && args[3].Kind == ValueInt && args[3].Int > 0 {
				query.Set("page", fmt.Sprint(args[3].Int))
			}
			if len(args) == 5 && args[4].Kind == ValueInt && args[4].Int > 0 {
				query.Set("pageSize", fmt.Sprint(args[4].Int))
			}
		} else if args[2].Kind == ValueInt {
			if args[2].Int > 0 {
				query.Set("page", fmt.Sprint(args[2].Int))
			}
			if len(args) == 4 && args[3].Kind == ValueInt && args[3].Int > 0 {
				query.Set("pageSize", fmt.Sprint(args[3].Int))
			}
		}
	}
	if encoded := query.Encode(); encoded != "" {
		pageURL += "?" + encoded
	}
	page := Object("ConnectApi.FollowingPage")
	page.Fields["currentPageUrl"] = String(pageURL)
	page.Fields["following"] = typedList("List<ConnectApi.Subscription>")
	page.Fields["total"] = Int(0)
	return page, nil
}

func (vm *VM) connectAPIChatterPostFeedElement(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 4 {
		return Null, fmt.Errorf("ConnectApi.ChatterFeeds.postFeedElement expects 2-4 arguments")
	}
	result := Object("ConnectApi.FeedElement")
	result.Fields["id"] = String("0D50000000000001")
	result.Fields["url"] = String("/services/data/vXX.X/connect/feed-elements/0D50000000000001")
	return result, nil
}

func (vm *VM) connectAPIChatterPostFeedElementBatch(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.ChatterFeeds.postFeedElementBatch expects 2 arguments")
	}
	results := typedList("List<ConnectApi.BatchResult>")
	result := Object("ConnectApi.BatchResult")
	result.Fields["id"] = String("0D50000000000001")
	result.Fields["url"] = String("/services/data/vXX.X/connect/feed-elements/0D50000000000001")
	result.Fields["isSuccess"] = Bool(true)
	result.Fields["statusCode"] = String("CREATED")
	results.List = append(results.List, result)
	return results, nil
}

func (vm *VM) connectAPIChatterUpdateComment(args []Value) (Value, error) {
	if len(args) != 3 {
		return Null, fmt.Errorf("ConnectApi.ChatterFeeds.updateComment expects 3 arguments")
	}
	comment := Object("ConnectApi.Comment")
	comment.Fields["id"] = String("0D50000000000001")
	comment.Fields["url"] = String("/services/data/vXX.X/connect/comments/0D50000000000001")
	return comment, nil
}

func (vm *VM) connectAPIChatterGetComment(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.ChatterFeeds.getComment expects 2 arguments")
	}
	comment := Object("ConnectApi.Comment")
	comment.Fields["id"] = String("0D50000000000001")
	comment.Fields["url"] = String("/services/data/vXX.X/connect/comments/0D50000000000001")
	return comment, nil
}

func (vm *VM) connectAPIChatterUsersSetPhoto(args []Value) (Value, error) {
	if len(args) < 3 || len(args) > 4 {
		return Null, fmt.Errorf("ConnectApi.ChatterUsers.setPhoto expects 3-4 arguments")
	}
	photo := Object("ConnectApi.Photo")
	photo.Fields["id"] = String(scalarText(args[1]))
	return photo, nil
}

func (vm *VM) connectAPIChatterUsersGetReputation(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.ChatterUsers.getReputation expects 2 arguments")
	}
	rep := Object("ConnectApi.Reputation")
	rep.Fields["id"] = String(scalarText(args[1]))
	return rep, nil
}
