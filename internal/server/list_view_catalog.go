package server

//go:generate python3 generate_list_view_catalog.py
//go:generate gofmt -w list_view_catalog_generated.go

type listViewColumnCatalogKey struct {
	apiVersion string
	object     string
	column     string
}
