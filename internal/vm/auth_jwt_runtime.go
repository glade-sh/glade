package vm

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

func newAuthJWT() Value {
	jwt := Object("Auth.JWT")
	jwt.Fields["nbfClockSkew"] = Int(30)
	jwt.Fields["validityLength"] = Int(300)
	jwt.Fields["additionalClaims"] = typedMap("Map<String,Object>")
	return jwt
}

func (vm *VM) callAuthJWTMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setIss", "setSub", "setAud":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Auth.JWT.%s expects String", method)
		}
		receiver.Fields[jwtFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setNbfClockSkew", "setValidityLength":
		if len(args) != 1 || (args[0].Kind != ValueInt && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Auth.JWT.%s expects Integer", method)
		}
		if jwtWasParsed(receiver) {
			return Null, receiver, false, true, newExceptionError("NoAccessException", "method is not available for a parsed JWT")
		}
		receiver.Fields[jwtFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setAdditionalClaims":
		if len(args) != 1 || (args[0].Kind != ValueMap && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Auth.JWT.setAdditionalClaims expects Map<String,Object>")
		}
		if args[0].Kind == ValueNull {
			receiver.Fields["additionalClaims"] = typedMap("Map<String,Object>")
		} else {
			receiver.Fields["additionalClaims"] = cloneValue(args[0])
		}
		return Null, receiver, true, true, nil
	case "getIss", "getSub", "getAud", "getNbfClockSkew", "getValidityLength":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Auth.JWT.%s expects 0 arguments", method)
		}
		if jwtWasParsed(receiver) && (method == "getNbfClockSkew" || method == "getValidityLength") {
			return Null, receiver, false, true, newExceptionError("NoAccessException", "method is not available for a parsed JWT")
		}
		field := jwtFieldName(method)
		if value, ok := receiver.Fields[field]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getAdditionalClaims":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Auth.JWT.getAdditionalClaims expects 0 arguments")
		}
		if value, ok := receiver.Fields["additionalClaims"]; ok && value.Kind == ValueMap {
			return value, receiver, false, true, nil
		}
		return typedMap("Map<String,Object>"), receiver, false, true, nil
	case "toJSONString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Auth.JWT.toJSONString expects 0 arguments")
		}
		claims := typedMap("Map<String,Object>")
		for _, field := range []string{"iss", "sub", "aud"} {
			if value, ok := receiver.Fields[field]; ok && value.Kind != ValueNull {
				jwtPutClaim(&claims, field, value)
			}
		}
		if !jwtWasParsed(receiver) {
			now := vm.fakeNow.Unix()
			jwtPutClaim(&claims, "iat", Int(now))
			jwtPutClaim(&claims, "nbf", Int(now-jwtIntegerField(receiver, "nbfClockSkew", 30)))
			jwtPutClaim(&claims, "exp", Int(now+jwtIntegerField(receiver, "validityLength", 300)))
			jwtPutClaim(&claims, "jti", String(vm.nextDeterministicUUID()))
		}
		if additional, ok := receiver.Fields["additionalClaims"]; ok && additional.Kind == ValueMap {
			for key, value := range additional.Map {
				claimKey := additional.MapKeys[key]
				if claimKey.Kind != ValueString {
					claimKey = valueFromMapKey(key)
				}
				if claimKey.Kind != ValueString || value.Kind == ValueNull {
					continue
				}
				jwtPutClaim(&claims, claimKey.Text, value)
			}
		}
		data, err := jsonMarshalNoEscape(jsonFromValue(claims, true))
		if err != nil {
			return Null, receiver, false, true, err
		}
		return String(string(data)), receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}

func jwtPutClaim(claims *Value, name string, value Value) {
	key := String(name)
	encoded := mapKey(key)
	claims.Map[encoded] = value
	claims.MapKeys[encoded] = key
}

func jwtIntegerField(jwt Value, field string, fallback int64) int64 {
	if value, ok := jwt.Fields[field]; ok && value.Kind == ValueInt {
		return value.Int
	}
	return fallback
}

func jwtWasParsed(jwt Value) bool {
	parsed, ok := jwt.Fields["__jwtParsed"]
	return ok && parsed.Kind == ValueBool && parsed.Bool
}

func jwtFieldName(method string) string {
	switch method {
	case "setNbfClockSkew", "getNbfClockSkew":
		return "nbfClockSkew"
	case "setValidityLength", "getValidityLength":
		return "validityLength"
	default:
		return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(method, "set"), "get"))
	}
}

func parseJWTFromStringWithoutValidation(value string) (Value, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Null, jwtValidationError()
	}
	headerPayload, err := decodeJWTPart(parts[0])
	if err != nil {
		return Null, jwtValidationError()
	}
	header, err := decodeJSONUntypedValue(string(headerPayload))
	if err != nil || header.Kind != ValueMap {
		return Null, jwtValidationError()
	}
	algorithm := header.Map[mapKey(String("alg"))]
	if algorithm.Kind != ValueString || strings.TrimSpace(algorithm.Text) == "" || strings.EqualFold(algorithm.Text, "none") {
		return Null, jwtValidationError()
	}
	if _, err := decodeJWTPart(parts[2]); err != nil {
		return Null, jwtValidationError()
	}
	payload, err := decodeJWTPart(parts[1])
	if err != nil {
		return Null, jwtValidationError()
	}
	decoded, err := decodeJSONUntypedValue(string(payload))
	if err != nil || decoded.Kind != ValueMap {
		return Null, jwtValidationError()
	}
	jwt := newAuthJWT()
	additional := typedMap("Map<String,Object>")
	for key, claim := range decoded.Map {
		claimName := decoded.MapKeys[key]
		if claimName.Kind != ValueString {
			claimName = valueFromMapKey(key)
		}
		if claimName.Kind != ValueString {
			continue
		}
		claimKey := strings.ToLower(claimName.Text)
		switch claimKey {
		case "iss", "sub", "aud":
			jwt.Fields[claimKey] = claim
		default:
			if claim.Kind == ValueList {
				data, marshalErr := jsonMarshalNoEscape(jsonFromValue(claim, true))
				if marshalErr != nil {
					return Null, fmt.Errorf("Auth.JWTUtil.parseJWTFromStringWithoutValidation claim %s: %w", claimName.Text, marshalErr)
				}
				claim = String(string(data))
			} else if claimKey == "nbf" || claimKey == "exp" || claimKey == "iat" {
				if numericDate, ok := jwtNumericDate(claim); ok {
					claim = numericDate
				}
			}
			encoded := mapKey(claimName)
			additional.Map[encoded] = claim
			additional.MapKeys[encoded] = claimName
		}
	}
	jwt.Fields["additionalClaims"] = additional
	jwt.Fields["__jwtParsed"] = Bool(true)
	return jwt, nil
}

func jwtValidationError() error {
	return newExceptionError("Auth.JWTValidationException", "We couldn’t parse the incomingJWT value. Check that you’re sending a JWT token.")
}

func jwtNumericDate(value Value) (Value, bool) {
	var milliseconds int64
	switch value.Kind {
	case ValueInt:
		milliseconds = value.Int * 1000
	case ValueDecimal:
		milliseconds = int64(value.Decimal * 1000)
	default:
		return Null, false
	}
	return platformScalar("Datetime", formatPlatformDatetime(time.UnixMilli(milliseconds).UTC())), true
}

func decodeJWTPart(value string) ([]byte, error) {
	for _, encoding := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("invalid base64url segment")
}
