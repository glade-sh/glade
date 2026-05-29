package apexast

type Visitor interface {
	VisitDeclaration(Declaration) bool
}

type VisitorFunc func(Declaration) bool

func (f VisitorFunc) VisitDeclaration(decl Declaration) bool {
	return f(decl)
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
