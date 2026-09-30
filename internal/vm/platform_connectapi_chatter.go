package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) connectAPIChatterUsersGetFollowings(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 5 {
		return Null, fmt.Errorf("ConnectApi.ChatterUsers.getFollowings expects 2-5 arguments")
	}
	if vm.testContext != nil && vm.testContext.SeeAllDataSet && !vm.testContext.SeeAllData {
		return Null, newExceptionError("UnsupportedOperationException", "ConnectApi.ChatterUsers.getFollowings requires SeeAllData=true in local tests")
	}
	pageURL := "/services/data/vXX.X/chatter/users/" + scalarText(args[1]) + "/following"
	query := ""
	if len(args) >= 3 {
		if args[2].Kind == ValueString {
			if len(args) == 5 && args[4].Kind == ValueInt && args[4].Int > 0 {
				query = "?pageSize=" + fmt.Sprint(args[4].Int) + "&filterType=" + args[2].Text
			} else if len(args) >= 4 && args[3].Kind == ValueInt && args[3].Int > 0 {
				query = "?page=" + fmt.Sprint(args[3].Int) + "&filterType=" + args[2].Text
			} else {
				query = "?filterType=" + args[2].Text
			}
		} else if args[2].Kind == ValueInt {
			if len(args) == 4 && args[3].Kind == ValueInt && args[3].Int > 0 {
				query = "?pageSize=" + fmt.Sprint(args[3].Int)
			} else if args[2].Int > 0 {
				query = "?page=" + fmt.Sprint(args[2].Int)
			}
		}
	}
	pageURL += query
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
	if feedItem, ok, err := vm.insertConnectAPIChatterFeedItem(args[1]); err != nil {
		return Null, err
	} else if ok {
		result := Object("ConnectApi.FeedElement")
		result.Fields["id"] = String(string(feedItem.ID))
		result.Fields["url"] = String("/services/data/vXX.X/connect/feed-elements/" + string(feedItem.ID))
		result.Fields["body"] = connectAPIChatterOutputBody(args[1])
		return result, nil
	}
	result := Object("ConnectApi.FeedElement")
	result.Fields["id"] = String("0D5000000000001")
	result.Fields["url"] = String("/services/data/vXX.X/connect/feed-elements/0D5000000000001")
	return result, nil
}

func (vm *VM) connectAPIChatterPostCommentToFeedElement(args []Value) (Value, error) {
	if len(args) != 3 && len(args) != 4 {
		return Null, fmt.Errorf("ConnectApi.ChatterFeeds.postCommentToFeedElement expects 3-4 arguments")
	}
	feedElementID := strings.TrimSpace(scalarText(args[1]))
	if feedElementID == "" {
		return Null, newExceptionError("ConnectApi.ConnectApiException", "The feedElementId parameter must be non-empty.")
	}
	body, ok := connectAPIChatterCommentBodyInput(args[2])
	if !ok {
		return Null, newExceptionError("ConnectApi.ConnectApiException", "The comment body parameter is required.")
	}
	if connectAPIChatterInvalidMention(body) {
		return Null, newExceptionError("ConnectApi.ConnectApiException", "Only user and group IDs may be used in inline mentions.")
	}
	bodyText := connectAPIChatterMessageBody(body)
	segments, hasSegments := objectFieldFold(body, "messageSegments")
	if !hasSegments || segments.Kind != ValueList || len(segments.List) == 0 {
		return Null, newExceptionError("ConnectApi.ConnectApiException", "The comment body parameter must be non-empty.")
	}
	if bodyText == "" {
		// A mention-only comment has no plain text but is still a valid Chatter body.
		bodyText = " "
	}
	if vm == nil || vm.Org == nil {
		comment := Object("ConnectApi.Comment")
		comment.Fields["id"] = String("0D7000000000001")
		comment.Fields["body"] = connectAPIChatterOutputMessageBody(body)
		return comment, nil
	}
	vm.ensureConnectAPIChatterParent(feedElementID)
	storage.EnsureStandardObject(vm.Org, "FeedComment")
	engine := vm.newDMLEngine(&Result{})
	results := engine.Insert([]storage.Record{{
		Object: "FeedComment",
		Fields: map[string]storage.Value{
			"FeedItemId":  storage.IDValue(storage.ID(feedElementID)),
			"CommentBody": storage.StringValue(bodyText),
			"CommentType": storage.StringValue("TextComment"),
		},
	}})
	if len(results) != 1 || !results[0].Success {
		message := "ConnectApi.ChatterFeeds.postCommentToFeedElement failed to create FeedComment"
		if len(results) == 1 && results[0].Error != "" {
			message = results[0].Error
		}
		return Null, newExceptionError("DmlException", message)
	}
	comment := Object("ConnectApi.Comment")
	comment.Fields["id"] = String(string(results[0].ID))
	comment.Fields["body"] = connectAPIChatterOutputMessageBody(body)
	return comment, nil
}

func connectAPIChatterCommentBodyInput(input Value) (Value, bool) {
	if input.Kind == ValueString {
		if strings.TrimSpace(input.Text) == "" {
			return Null, false
		}
		segment := Object("ConnectApi.TextSegmentInput")
		segment.Fields["text"] = input
		body := Object("ConnectApi.MessageBodyInput")
		body.Fields["messageSegments"] = List(segment)
		return body, true
	}
	if input.Kind != ValueObject {
		return Null, false
	}
	body, ok := objectFieldFold(input, "body")
	if !ok || body.Kind != ValueObject {
		return Null, false
	}
	return body, true
}

func (vm *VM) insertConnectAPIChatterFeedItem(input Value) (storage.Record, bool, error) {
	if vm == nil || vm.Org == nil || input.Kind != ValueObject {
		return storage.Record{}, false, nil
	}
	subject, subjectOK := objectFieldFold(input, "subjectId")
	body, bodyOK := objectFieldFold(input, "body")
	if !subjectOK || !bodyOK || subject.Kind == ValueNull || body.Kind == ValueNull {
		return storage.Record{}, false, nil
	}
	parentID := scalarText(subject)
	if strings.EqualFold(strings.TrimSpace(parentID), "me") {
		parentID = vm.currentUserID()
		if parentID == "" {
			parentID = "005000000000001"
		}
	}
	if strings.TrimSpace(parentID) == "" {
		return storage.Record{}, false, nil
	}
	if invalidMention := connectAPIChatterInvalidMention(body); invalidMention {
		return storage.Record{}, false, newExceptionError("ConnectApi.ConnectApiException", "Only user and group IDs may be used in inline mentions.")
	}
	bodyText := connectAPIChatterMessageBody(body)
	segments, hasSegments := objectFieldFold(body, "messageSegments")
	if !hasSegments || segments.Kind != ValueList || len(segments.List) == 0 {
		return storage.Record{}, false, nil
	}
	if bodyText == "" {
		// A mention-only post has no plain text but is still a valid Chatter body.
		bodyText = " "
	}
	vm.ensureConnectAPIChatterParent(parentID)
	storage.EnsureStandardObject(vm.Org, "FeedItem")
	engine := vm.newDMLEngine(&Result{})
	results := engine.Insert([]storage.Record{{
		Object: "FeedItem",
		Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(storage.ID(parentID)),
			"Body":     storage.StringValue(bodyText),
			"Type":     storage.StringValue("TextPost"),
		},
	}})
	if len(results) != 1 {
		return storage.Record{}, false, fmt.Errorf("ConnectApi.ChatterFeeds.postFeedElement failed to create FeedItem")
	}
	if !results[0].Success {
		if results[0].Error != "" {
			return storage.Record{}, false, newExceptionError("DmlException", results[0].Error)
		}
		return storage.Record{}, false, newExceptionError("DmlException", "ConnectApi.ChatterFeeds.postFeedElement failed to create FeedItem")
	}
	stored, ok := vm.Org.Objects["FeedItem"].Records[results[0].ID]
	if !ok {
		stored = storage.Record{Object: "FeedItem", ID: results[0].ID}
	}
	return stored, true, nil
}

func connectAPIChatterInvalidMention(body Value) bool {
	segments, ok := objectFieldFold(body, "messageSegments")
	if !ok || segments.Kind != ValueList {
		return false
	}
	for _, segment := range segments.List {
		if segment.Kind != ValueObject || !strings.HasSuffix(strings.ToLower(segment.Type), "mentionsegmentinput") {
			continue
		}
		id, ok := objectFieldFold(segment, "id")
		if !ok {
			continue
		}
		value := scalarText(id)
		if len(value) < 3 || (value[:3] != "005" && value[:3] != "0F9") {
			return true
		}
	}
	return false
}

func (vm *VM) ensureConnectAPIChatterParent(parentID string) {
	if vm == nil || vm.Org == nil || len(parentID) < 3 {
		return
	}
	objectName, ok := vm.sObjectNameForIDPrefix(parentID[:3])
	if !ok {
		return
	}
	storage.EnsureStandardObject(vm.Org, objectName)
	object := vm.Org.Objects[objectName]
	if _, _, found := storage.LookupRecordByID(object.Records, storage.ID(parentID)); found {
		return
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	object.Records[storage.ID(parentID)] = storage.Record{Object: objectName, ID: storage.ID(parentID)}
	vm.Org.Objects[objectName] = object
}

func connectAPIChatterMessageBody(body Value) string {
	segments, ok := objectFieldFold(body, "messageSegments")
	if !ok || segments.Kind != ValueList {
		return ""
	}
	var rendered strings.Builder
	for _, segment := range segments.List {
		if segment.Kind != ValueObject {
			continue
		}
		if text, ok := objectFieldFold(segment, "text"); ok && text.Kind == ValueString {
			rendered.WriteString(text.Text)
			continue
		}
		tag := connectAPIChatterMarkupTag(segment)
		if tag == "" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(segment.Type), "markupendsegmentinput") {
			rendered.WriteString("</" + tag + ">")
		} else {
			rendered.WriteString("<" + tag + ">")
		}
	}
	return rendered.String()
}

func connectAPIChatterOutputBody(input Value) Value {
	body, ok := objectFieldFold(input, "body")
	if !ok || body.Kind != ValueObject {
		return Null
	}
	return connectAPIChatterOutputMessageBody(body)
}

func connectAPIChatterOutputMessageBody(body Value) Value {
	segments, ok := objectFieldFold(body, "messageSegments")
	if !ok || segments.Kind != ValueList {
		return Null
	}
	out := Object("ConnectApi.FeedBody")
	outSegments := make([]Value, 0, len(segments.List))
	for _, segment := range segments.List {
		if segment.Kind != ValueObject {
			continue
		}
		switch {
		case strings.HasSuffix(strings.ToLower(segment.Type), "textsegmentinput"):
			if text, ok := objectFieldFold(segment, "text"); ok && text.Kind == ValueString {
				outSegments = append(outSegments, connectAPIChatterOutputTextSegments(text.Text)...)
			}
		case strings.HasSuffix(strings.ToLower(segment.Type), "mentionsegmentinput"):
			if id, ok := objectFieldFold(segment, "id"); ok {
				reference := Object("ConnectApi.Reference")
				reference.Fields["id"] = id
				mention := Object("ConnectApi.MentionSegment")
				mention.Fields["record"] = reference
				outSegments = append(outSegments, mention)
			}
		case strings.HasSuffix(strings.ToLower(segment.Type), "hashtagsegmentinput"):
			if tag, ok := objectFieldFold(segment, "tag"); ok {
				hashtag := Object("ConnectApi.HashtagSegment")
				hashtag.Fields["tag"] = tag
				outSegments = append(outSegments, hashtag)
			}
		case strings.HasSuffix(strings.ToLower(segment.Type), "linksegmentinput"):
			if url, ok := objectFieldFold(segment, "url"); ok {
				link := Object("ConnectApi.LinkSegment")
				link.Fields["url"] = url
				outSegments = append(outSegments, link)
			}
		case strings.HasSuffix(strings.ToLower(segment.Type), "markupbeginsegmentinput"):
			if markup, ok := objectFieldFold(segment, "markupType"); ok {
				begin := Object("ConnectApi.MarkupBeginSegment")
				begin.Fields["markupType"] = markup
				outSegments = append(outSegments, begin)
			}
		case strings.HasSuffix(strings.ToLower(segment.Type), "markupendsegmentinput"):
			if markup, ok := objectFieldFold(segment, "markupType"); ok {
				end := Object("ConnectApi.MarkupEndSegment")
				end.Fields["markupType"] = markup
				outSegments = append(outSegments, end)
			}
		}
	}
	out.Fields["messageSegments"] = List(outSegments...)
	return out
}

func connectAPIChatterOutputTextSegments(text string) []Value {
	segments := make([]Value, 0, 3)
	for len(text) > 0 {
		urlStart, urlEnd := connectAPIChatterURLRange(text)
		hashtagStart, hashtagEnd := connectAPIChatterHashtagRange(text)
		start, end, kind := -1, -1, ""
		if urlStart >= 0 && (hashtagStart < 0 || urlStart < hashtagStart) {
			start, end, kind = urlStart, urlEnd, "url"
		} else if hashtagStart >= 0 {
			start, end, kind = hashtagStart, hashtagEnd, "hashtag"
		}
		if start < 0 {
			segments = append(segments, connectAPIChatterTextSegment(text))
			break
		}
		if start > 0 {
			segments = append(segments, connectAPIChatterTextSegment(text[:start]))
		}
		if kind == "url" {
			link := Object("ConnectApi.LinkSegment")
			link.Fields["url"] = String(text[start:end])
			segments = append(segments, link)
		} else {
			hashtag := Object("ConnectApi.HashtagSegment")
			hashtag.Fields["tag"] = String(text[start+1 : end])
			segments = append(segments, hashtag)
		}
		text = text[end:]
	}
	return segments
}

func connectAPIChatterTextSegment(text string) Value {
	segment := Object("ConnectApi.TextSegment")
	segment.Fields["text"] = String(text)
	return segment
}

func connectAPIChatterURLRange(text string) (int, int) {
	start := strings.Index(text, "http://")
	secureStart := strings.Index(text, "https://")
	if start < 0 || (secureStart >= 0 && secureStart < start) {
		start = secureStart
	}
	if start < 0 {
		return -1, -1
	}
	end := start
	for end < len(text) && !strings.ContainsRune(" \t\r\n<>,", rune(text[end])) {
		end++
	}
	return start, end
}

func connectAPIChatterHashtagRange(text string) (int, int) {
	for offset := 0; offset < len(text); {
		index := strings.IndexByte(text[offset:], '#')
		if index < 0 {
			return -1, -1
		}
		start := offset + index
		if start+1 < len(text) && isConnectAPIChatterWord(text[start+1]) {
			end := start + 1
			for end < len(text) && isConnectAPIChatterWord(text[end]) {
				end++
			}
			return start, end
		}
		offset = start + 1
	}
	return -1, -1
}

func isConnectAPIChatterWord(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}

func connectAPIChatterMarkupTag(segment Value) string {
	markup, ok := objectFieldFold(segment, "markupType")
	if !ok {
		return ""
	}
	name := strings.ToLower(strings.TrimSpace(markup.Text))
	switch name {
	case "bold":
		return "b"
	case "code":
		return "code"
	case "italic":
		return "i"
	case "listitem":
		return "li"
	case "orderedlist":
		return "ol"
	case "paragraph":
		return "p"
	case "strikethrough":
		return "s"
	case "underline":
		return "u"
	case "unorderedlist":
		return "ul"
	default:
		return ""
	}
}

func (vm *VM) connectAPIChatterPostFeedElementBatch(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.ChatterFeeds.postFeedElementBatch expects 2 arguments")
	}
	results := typedList("List<ConnectApi.BatchResult>")
	result := Object("ConnectApi.BatchResult")
	result.Fields["id"] = String("0D5000000000001")
	result.Fields["url"] = String("/services/data/vXX.X/connect/feed-elements/0D5000000000001")
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
	comment.Fields["id"] = String("0D7000000000001")
	comment.Fields["url"] = String("/services/data/vXX.X/connect/comments/0D7000000000001")
	return comment, nil
}

func (vm *VM) connectAPIChatterGetComment(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("ConnectApi.ChatterFeeds.getComment expects 2 arguments")
	}
	comment := Object("ConnectApi.Comment")
	comment.Fields["id"] = String("0D7000000000001")
	comment.Fields["url"] = String("/services/data/vXX.X/connect/comments/0D7000000000001")
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
