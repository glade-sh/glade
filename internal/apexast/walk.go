package apexast

type Visitor interface {
	VisitDeclaration(decl Declaration) bool
}

type VisitorFunc func(decl Declaration) bool

func (fn VisitorFunc) VisitDeclaration(decl Declaration) bool {
	return fn(decl)
}

func WalkFile(file File, visitor Visitor) {
	for _, decl := range file.Declarations {
		WalkDeclaration(decl, visitor)
	}
}

func WalkDeclaration(decl Declaration, visitor Visitor) {
	if !visitor.VisitDeclaration(decl) {
		return
	}
	for _, child := range decl.Members {
		WalkDeclaration(child, visitor)
	}
}
