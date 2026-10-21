package vm

import "strings"

func initializeAuthResponseConstructor(object *Value, ctor Method, args []Value) {
	if !ctor.IsConstructor || len(ctor.Program.Instructions) != 0 {
		return
	}
	var fields []string
	switch strings.ToLower(ctor.ClassName) {
	case "auth.authprovidertokenresponse":
		if len(args) != 4 {
			return
		}
		fields = []string{"provider", "oauthToken", "oauthSecretOrRefreshToken", "state"}
	case "auth.oauthrefreshresult":
		if len(args) != 2 && len(args) != 3 {
			return
		}
		fields = []string{"accessToken", "refreshToken", "error"}
	default:
		return
	}
	for i, arg := range args {
		object.Fields[fields[i]] = arg
	}
}
