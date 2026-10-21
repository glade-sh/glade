package soql

import "fmt"

// SemiJoinError keeps captured semi-join contracts distinct from ordinary
// parser recovery. Dynamic callers still unwrap the native QueryException.
type SemiJoinError struct {
	*QueryError
	CompileObject  string
	CompileMessage string
}

func (e *SemiJoinError) Unwrap() error { return e.QueryError }

func semiJoinError(message string) *SemiJoinError {
	return &SemiJoinError{QueryError: &QueryError{Message: message, Err: fmt.Errorf("soql: %s", message)}}
}

// Only an OR ancestor of a semi-join is prohibited. Scalar OR siblings under
// the outer AND are accepted. Parentheses around the whole WHERE expression
// preserve its top level; a descendant parenthesized AND containing the join
// is the captured nested-WHERE rejection.
func semiJoinBooleanRejection(condition *Condition) error {
	var walk func(*Condition, bool, bool, bool) error
	walk = func(current *Condition, underOr, nestedAnd, ungroupedAnd bool) error {
		if current == nil {
			return nil
		}
		underOr = underOr || len(current.Or) > 0
		nestedAnd = nestedAnd || current != condition && current.Parenthesized && current.SourceAnd
		ungroupedAnd = ungroupedAnd || underOr && current.SourceAnd && !current.Parenthesized
		if current.Subquery != nil {
			if underOr {
				if ungroupedAnd {
					// SOQL selection R160 has scalar AND semi-join OR scalar, without
					// grouping the AND operand. Its OR syntax rejection
					// precedes the direct semi-join OR contract.
					return unexpectedToken("OR", "semi-join and anti-join subqueries cannot be combined with OR")
				}
				return semiJoinError("Semi join sub-selects are not allowed with the 'OR' operator")
			}
			if nestedAnd {
				return semiJoinError("Semi join sub-selects are only allowed at the top level WHERE expressions and not in nested WHERE expressions.")
			}
		}
		for i := range current.And {
			if err := walk(&current.And[i], underOr, nestedAnd, ungroupedAnd); err != nil {
				return err
			}
		}
		for i := range current.Or {
			if err := walk(&current.Or[i], underOr, nestedAnd, ungroupedAnd); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(condition, false, false, false)
}
