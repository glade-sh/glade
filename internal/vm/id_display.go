package vm

func originalIDDisplayText(value Value) (string, bool) {
	if !value.idOriginalDisplay {
		return "", false
	}
	text, ok := typedIDValueText(value)
	if !ok {
		return "", false
	}
	if len(text) == 18 {
		text = text[:15]
	}
	return text, true
}
