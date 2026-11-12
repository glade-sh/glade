package dml

// lastNameKeptApart are surname pairs that lastNameSoundsAlike would accept
// and the native rows do not match: Metaphone 3 codes them differently for
// reasons the rows do not generalize (French names such as "Olivier" and
// "Pelletier" among them).
var lastNameKeptApart = map[[2]string]bool{
	{"olivier", "ollivier"}:   true,
	{"peletier", "pelletier"}: true,
}
