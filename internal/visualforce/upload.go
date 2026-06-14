package visualforce

import (
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

type InputFileUpload struct {
	FieldName   string `json:"fieldName,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Blob        []byte `json:"-"`
}

type InputFileUploadBinding struct {
	FieldName         string
	BlobTarget        string
	FileNameTarget    string
	ContentTypeTarget string
	SizeTarget        string
}

func InputFileUploadBindingsFromMarkup(source string) ([]InputFileUploadBinding, error) {
	root, err := ParseMarkupTree(source)
	if err != nil {
		return nil, err
	}
	var bindings []InputFileUploadBinding
	collectInputFileUploadBindings(root, &bindings)
	return bindings, nil
}

func BindInputFileUploadAssignments(req *http.Request, bindings []InputFileUploadBinding) (map[string]any, error) {
	fieldNames := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if fieldName := strings.TrimSpace(binding.FieldName); fieldName != "" {
			fieldNames = append(fieldNames, fieldName)
		}
	}
	uploads, err := readInputFileUploads(req, fieldNames, false)
	if err != nil {
		return nil, err
	}
	assignments := make(map[string]any)
	for _, binding := range bindings {
		fieldName := strings.TrimSpace(binding.FieldName)
		upload, ok := uploads[fieldName]
		if !ok {
			return nil, fmt.Errorf("visualforce inputFile upload part %s not found", fieldName)
		}
		assignInputFileUploadValue(assignments, binding.BlobTarget, upload.Blob)
		assignInputFileUploadValue(assignments, binding.FileNameTarget, upload.FileName)
		assignInputFileUploadValue(assignments, binding.ContentTypeTarget, upload.ContentType)
		assignInputFileUploadValue(assignments, binding.SizeTarget, upload.Size)
	}
	return assignments, nil
}

func BindInputFileUpload(req *http.Request, fieldName string) (InputFileUpload, error) {
	fieldName = strings.TrimSpace(fieldName)
	uploads, err := readInputFileUploads(req, []string{fieldName}, fieldName == "")
	if err != nil {
		return InputFileUpload{}, err
	}
	if fieldName == "" {
		for _, upload := range uploads {
			return upload, nil
		}
		return InputFileUpload{}, fmt.Errorf("visualforce inputFile upload part not found")
	}
	if upload, ok := uploads[fieldName]; ok {
		return upload, nil
	}
	return InputFileUpload{}, fmt.Errorf("visualforce inputFile upload part %s not found", fieldName)
}

func renderApexInputFile(node *MarkupNode, _ *RenderContext) (string, error) {
	fieldName := inputFileUploadFieldName(node)
	if fieldName == "" {
		fieldName = "file"
	}
	id := strings.TrimSpace(node.Attribute("id"))
	if id == "" {
		id = fieldName
	}
	attrs := strings.Builder{}
	attrs.WriteString(` type="file" name="`)
	attrs.WriteString(html.EscapeString(fieldName))
	attrs.WriteString(`" id="`)
	attrs.WriteString(html.EscapeString(id))
	attrs.WriteString(`"`)
	if accept := strings.TrimSpace(node.Attribute("accept")); accept != "" {
		attrs.WriteString(` accept="`)
		attrs.WriteString(html.EscapeString(accept))
		attrs.WriteString(`"`)
	}
	if className := strings.TrimSpace(node.Attribute("styleclass")); className != "" {
		attrs.WriteString(` class="`)
		attrs.WriteString(html.EscapeString(className))
		attrs.WriteString(`"`)
	}
	if style := strings.TrimSpace(node.Attribute("style")); style != "" {
		attrs.WriteString(` style="`)
		attrs.WriteString(html.EscapeString(style))
		attrs.WriteString(`"`)
	}
	if isTruthyAttribute(node.Attribute("disabled")) {
		attrs.WriteString(` disabled="disabled"`)
	}
	if isTruthyAttribute(node.Attribute("required")) {
		attrs.WriteString(` required="required"`)
	}
	return `<input` + attrs.String() + ` />`, nil
}

func readInputFileUploads(req *http.Request, fieldNames []string, allowAny bool) (map[string]InputFileUpload, error) {
	if req == nil {
		return nil, fmt.Errorf("visualforce inputFile upload request is nil")
	}
	if req.MultipartForm != nil {
		return readInputFileUploadsFromMultipartForm(req.MultipartForm, fieldNames, allowAny)
	}
	reader, err := req.MultipartReader()
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(fieldNames))
	for _, fieldName := range fieldNames {
		if fieldName = strings.TrimSpace(fieldName); fieldName != "" {
			wanted[fieldName] = true
		}
	}
	uploads := make(map[string]InputFileUpload)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		formName := part.FormName()
		if !allowAny && !wanted[formName] {
			continue
		}
		if part.FileName() == "" {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(part, int64(MaxVisualforceUploadBytes)+1))
		if err != nil {
			return nil, err
		}
		if err := CheckVisualforceUploadSize(len(body)); err != nil {
			return nil, err
		}
		uploads[formName] = InputFileUpload{
			FieldName:   formName,
			FileName:    part.FileName(),
			ContentType: strings.TrimSpace(part.Header.Get("Content-Type")),
			Size:        int64(len(body)),
			Blob:        body,
		}
		if allowAny || len(uploads) == len(wanted) {
			break
		}
	}
	return uploads, nil
}

func readInputFileUploadsFromMultipartForm(form *multipart.Form, fieldNames []string, allowAny bool) (map[string]InputFileUpload, error) {
	if form == nil {
		return nil, fmt.Errorf("visualforce inputFile upload form is nil")
	}
	wanted := make(map[string]bool, len(fieldNames))
	for _, fieldName := range fieldNames {
		if fieldName = strings.TrimSpace(fieldName); fieldName != "" {
			wanted[fieldName] = true
		}
	}
	uploads := make(map[string]InputFileUpload)
	for formName, files := range form.File {
		if !allowAny && !wanted[formName] {
			continue
		}
		if len(files) == 0 {
			continue
		}
		fileHeader := files[0]
		file, err := fileHeader.Open()
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(file, int64(MaxVisualforceUploadBytes)+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if err := CheckVisualforceUploadSize(len(body)); err != nil {
			return nil, err
		}
		uploads[formName] = InputFileUpload{
			FieldName:   formName,
			FileName:    fileHeader.Filename,
			ContentType: strings.TrimSpace(fileHeader.Header.Get("Content-Type")),
			Size:        int64(len(body)),
			Blob:        body,
		}
		if allowAny || len(uploads) == len(wanted) {
			break
		}
	}
	return uploads, nil
}

func collectInputFileUploadBindings(node *MarkupNode, bindings *[]InputFileUploadBinding) {
	if node == nil {
		return
	}
	if node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") && strings.EqualFold(node.Name, "inputfile") {
		binding := InputFileUploadBinding{
			FieldName:         inputFileUploadFieldName(node),
			BlobTarget:        visualforceBindingTarget(node.Attribute("value")),
			FileNameTarget:    visualforceBindingTarget(node.Attribute("filename")),
			ContentTypeTarget: visualforceBindingTarget(node.Attribute("contenttype")),
			SizeTarget:        visualforceBindingTarget(inputFileUploadSizeAttribute(node)),
		}
		*bindings = append(*bindings, binding)
	}
	for _, child := range node.Children {
		collectInputFileUploadBindings(child, bindings)
	}
}

func inputFileUploadFieldName(node *MarkupNode) string {
	if node == nil {
		return ""
	}
	if id := strings.TrimSpace(node.Attribute("id")); id != "" {
		return id
	}
	if target := visualforceBindingTarget(node.Attribute("value")); target != "" {
		parts := strings.Split(target, ".")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}

func visualforceBindingTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{!") && strings.HasSuffix(raw, "}") {
		raw = strings.TrimSpace(raw[2 : len(raw)-1])
	}
	return raw
}

func inputFileUploadSizeAttribute(node *MarkupNode) string {
	if node == nil {
		return ""
	}
	if raw := strings.TrimSpace(node.Attribute("filesize")); raw != "" {
		return raw
	}
	return node.Attribute("size")
}

func assignInputFileUploadValue(assignments map[string]any, target string, value any) {
	target = strings.TrimSpace(target)
	if target == "" {
		return
	}
	assignments[target] = value
}

func isTruthyAttribute(raw string) bool {
	raw = strings.TrimSpace(raw)
	return raw == "true" || raw == "1" || strings.EqualFold(raw, "on")
}
