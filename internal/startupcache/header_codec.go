package startupcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const (
	testCacheHeaderEncodingVersion = 1
	maxTestCacheHeaderBytes        = 16 << 20
	maxTestCacheHeaderStringBytes  = 1 << 20
	maxTestCacheHeaderPathBytes    = 64 << 10
	maxTestCacheHeaderItems        = 100_000
	maxEncodedTestCacheHeaderBytes = maxTestCacheHeaderBytes + 8 + 2 + 4 + sha256.Size
)

var testCacheHeaderMagic = [8]byte{'G', 'L', 'A', 'D', 'E', 'H', 'D', 'R'}

type testCacheHeaderWriter struct {
	body []byte
	err  error
}

func marshalTestCacheHeader(header testCacheHeader) ([]byte, error) {
	writer := testCacheHeaderWriter{body: make([]byte, 0, testCacheHeaderBodyCapacity(header))}
	writer.signed(int64(header.FormatVersion))
	writer.signed(int64(header.Version))
	writer.path(header.ProjectRoot)
	writer.string(header.BuiltAt)
	writer.string(header.PlatformABI)
	writer.string(header.RuntimeABI)
	writer.string(header.RuntimeKey)
	writer.manifest(header.Manifest)
	writer.path(header.PayloadFile)
	writer.hash(header.PayloadSHA256)
	writer.signed(header.PayloadSize)
	if writer.err != nil {
		return nil, writer.err
	}
	if len(writer.body) > maxTestCacheHeaderBytes {
		return nil, fmt.Errorf("startup cache header body is too large: %d bytes", len(writer.body))
	}

	data := make([]byte, 0, len(testCacheHeaderMagic)+2+4+len(writer.body)+sha256.Size)
	data = append(data, testCacheHeaderMagic[:]...)
	var fixed [4]byte
	binary.BigEndian.PutUint16(fixed[:2], testCacheHeaderEncodingVersion)
	data = append(data, fixed[:2]...)
	binary.BigEndian.PutUint32(fixed[:], uint32(len(writer.body)))
	data = append(data, fixed[:]...)
	data = append(data, writer.body...)
	sum := sha256.Sum256(data)
	data = append(data, sum[:]...)
	return data, nil
}

func testCacheHeaderBodyCapacity(header testCacheHeader) int {
	capacity := 512
	add := func(count, bytesPerItem int) {
		if count <= 0 || capacity == maxTestCacheHeaderBytes {
			return
		}
		if count > (maxTestCacheHeaderBytes-capacity)/bytesPerItem {
			capacity = maxTestCacheHeaderBytes
			return
		}
		capacity += count * bytesPerItem
	}
	add(len(header.Manifest.Features), 32)
	add(len(header.Manifest.Files)+len(header.Manifest.ConfigFiles), 128)
	add(len(header.Manifest.PackageRoots), 64)
	return capacity
}

func (w *testCacheHeaderWriter) manifest(manifest Manifest) {
	w.signed(int64(manifest.SchemaVersion))
	w.path(manifest.ProjectRoot)
	w.string(manifest.SourceAPIVersion)
	w.string(manifest.Namespace)
	w.hash(manifest.ProjectDigest)
	w.strings(manifest.Features)
	w.boolean(manifest.Complete)
	w.files(manifest.Files)
	w.files(manifest.ConfigFiles)
	w.directories(manifest.PackageRoots)
}

func (w *testCacheHeaderWriter) files(files []File) {
	if w.err != nil {
		return
	}
	if files == nil {
		w.boolean(false)
		return
	}
	w.boolean(true)
	w.count(len(files))
	for _, file := range files {
		w.path(file.Path)
		w.signed(file.Size)
		w.signed(file.ModTime)
		w.hash(file.SHA256)
	}
}

func (w *testCacheHeaderWriter) directories(directories []Directory) {
	if w.err != nil {
		return
	}
	if directories == nil {
		w.boolean(false)
		return
	}
	w.boolean(true)
	w.count(len(directories))
	for _, directory := range directories {
		w.path(directory.Path)
		w.signed(directory.ModTime)
	}
}

func (w *testCacheHeaderWriter) strings(values []string) {
	if w.err != nil {
		return
	}
	if values == nil {
		w.boolean(false)
		return
	}
	w.boolean(true)
	w.count(len(values))
	for _, value := range values {
		w.string(value)
	}
}

func (w *testCacheHeaderWriter) count(value int) {
	if w.err != nil {
		return
	}
	if value < 0 || value > maxTestCacheHeaderItems {
		w.err = fmt.Errorf("startup cache header item count %d exceeds limit", value)
		return
	}
	w.unsigned(uint64(value))
}

func (w *testCacheHeaderWriter) path(value string) {
	w.limitedString(value, maxTestCacheHeaderPathBytes, "path")
}

func (w *testCacheHeaderWriter) string(value string) {
	w.limitedString(value, maxTestCacheHeaderStringBytes, "string")
}

func (w *testCacheHeaderWriter) limitedString(value string, limit int, kind string) {
	if w.err != nil {
		return
	}
	if len(value) > limit {
		w.err = fmt.Errorf("startup cache header %s is too long: %d bytes", kind, len(value))
		return
	}
	w.unsigned(uint64(len(value)))
	w.appendString(value)
}

func (w *testCacheHeaderWriter) hash(value string) {
	if w.err != nil {
		return
	}
	if value == "" {
		w.byte(0)
		return
	}
	if len(value) == sha256HexLength && isLowerHex(value) {
		var raw [sha256.Size]byte
		for i := range raw {
			raw[i] = lowerHexNibble(value[i*2])<<4 | lowerHexNibble(value[i*2+1])
		}
		w.byte(1)
		w.appendBytes(raw[:])
		return
	}
	w.byte(2)
	w.string(value)
}

func lowerHexNibble(value byte) byte {
	if value <= '9' {
		return value - '0'
	}
	return value - 'a' + 10
}

func (w *testCacheHeaderWriter) boolean(value bool) {
	if w.err != nil {
		return
	}
	if value {
		w.byte(1)
	} else {
		w.byte(0)
	}
}

func (w *testCacheHeaderWriter) signed(value int64) {
	if w.err != nil {
		return
	}
	var encoded [binary.MaxVarintLen64]byte
	n := binary.PutVarint(encoded[:], value)
	w.appendBytes(encoded[:n])
}

func (w *testCacheHeaderWriter) unsigned(value uint64) {
	if w.err != nil {
		return
	}
	var encoded [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(encoded[:], value)
	w.appendBytes(encoded[:n])
}

func (w *testCacheHeaderWriter) byte(value byte) {
	if w.err != nil {
		return
	}
	if len(w.body) == maxTestCacheHeaderBytes {
		w.err = fmt.Errorf("startup cache header body exceeds %d bytes", maxTestCacheHeaderBytes)
		return
	}
	w.body = append(w.body, value)
}

func (w *testCacheHeaderWriter) appendString(value string) {
	if w.err != nil {
		return
	}
	if len(value) > maxTestCacheHeaderBytes-len(w.body) {
		w.err = fmt.Errorf("startup cache header body exceeds %d bytes", maxTestCacheHeaderBytes)
		return
	}
	w.body = append(w.body, value...)
}

func (w *testCacheHeaderWriter) appendBytes(value []byte) {
	if w.err != nil {
		return
	}
	if len(value) > maxTestCacheHeaderBytes-len(w.body) {
		w.err = fmt.Errorf("startup cache header body exceeds %d bytes", maxTestCacheHeaderBytes)
		return
	}
	w.body = append(w.body, value...)
}

type testCacheHeaderReader struct {
	body   []byte
	offset int
}

func unmarshalTestCacheHeader(data []byte) (testCacheHeader, error) {
	const minimumSize = 8 + 2 + 4 + sha256.Size
	if len(data) < minimumSize || len(data) > maxEncodedTestCacheHeaderBytes {
		return testCacheHeader{}, errMalformedTestCacheHeader
	}
	if !bytes.Equal(data[:len(testCacheHeaderMagic)], testCacheHeaderMagic[:]) {
		return testCacheHeader{}, errMalformedTestCacheHeader
	}
	offset := len(testCacheHeaderMagic)
	if version := binary.BigEndian.Uint16(data[offset : offset+2]); version != testCacheHeaderEncodingVersion {
		return testCacheHeader{}, errMalformedTestCacheHeader
	}
	offset += 2
	bodySize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if bodySize > maxTestCacheHeaderBytes || len(data) != offset+bodySize+sha256.Size {
		return testCacheHeader{}, errMalformedTestCacheHeader
	}
	wantChecksum := data[offset+bodySize:]
	gotChecksum := sha256.Sum256(data[:offset+bodySize])
	if !bytes.Equal(wantChecksum, gotChecksum[:]) {
		return testCacheHeader{}, errMalformedTestCacheHeader
	}

	reader := testCacheHeaderReader{body: data[offset : offset+bodySize]}
	formatVersion, err := reader.signed()
	if err != nil {
		return testCacheHeader{}, err
	}
	version, err := reader.signed()
	if err != nil {
		return testCacheHeader{}, err
	}
	projectRoot, err := reader.path()
	if err != nil {
		return testCacheHeader{}, err
	}
	builtAt, err := reader.string()
	if err != nil {
		return testCacheHeader{}, err
	}
	platformABI, err := reader.string()
	if err != nil {
		return testCacheHeader{}, err
	}
	runtimeABI, err := reader.string()
	if err != nil {
		return testCacheHeader{}, err
	}
	runtimeKey, err := reader.string()
	if err != nil {
		return testCacheHeader{}, err
	}
	manifest, err := reader.manifest()
	if err != nil {
		return testCacheHeader{}, err
	}
	payloadFile, err := reader.path()
	if err != nil {
		return testCacheHeader{}, err
	}
	payloadSHA256, err := reader.hash()
	if err != nil {
		return testCacheHeader{}, err
	}
	payloadSize, err := reader.signed()
	if err != nil {
		return testCacheHeader{}, err
	}
	if reader.remaining() != 0 {
		return testCacheHeader{}, errMalformedTestCacheHeader
	}
	return testCacheHeader{
		FormatVersion: int(formatVersion),
		Version:       int(version),
		ProjectRoot:   projectRoot,
		BuiltAt:       builtAt,
		PlatformABI:   platformABI,
		RuntimeABI:    runtimeABI,
		RuntimeKey:    runtimeKey,
		Manifest:      manifest,
		PayloadFile:   payloadFile,
		PayloadSHA256: payloadSHA256,
		PayloadSize:   payloadSize,
	}, nil
}

func (r *testCacheHeaderReader) manifest() (Manifest, error) {
	schemaVersion, err := r.signed()
	if err != nil {
		return Manifest{}, err
	}
	projectRoot, err := r.path()
	if err != nil {
		return Manifest{}, err
	}
	sourceAPIVersion, err := r.string()
	if err != nil {
		return Manifest{}, err
	}
	namespace, err := r.string()
	if err != nil {
		return Manifest{}, err
	}
	projectDigest, err := r.hash()
	if err != nil {
		return Manifest{}, err
	}
	features, err := r.strings()
	if err != nil {
		return Manifest{}, err
	}
	complete, err := r.boolean()
	if err != nil {
		return Manifest{}, err
	}
	files, err := r.files()
	if err != nil {
		return Manifest{}, err
	}
	configFiles, err := r.files()
	if err != nil {
		return Manifest{}, err
	}
	packageRoots, err := r.directories()
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{
		SchemaVersion:    int(schemaVersion),
		ProjectRoot:      projectRoot,
		SourceAPIVersion: sourceAPIVersion,
		Namespace:        namespace,
		ProjectDigest:    projectDigest,
		Features:         features,
		Complete:         complete,
		Files:            files,
		ConfigFiles:      configFiles,
		PackageRoots:     packageRoots,
	}, nil
}

func (r *testCacheHeaderReader) files() ([]File, error) {
	present, err := r.boolean()
	if err != nil || !present {
		return nil, err
	}
	count, err := r.count()
	if err != nil {
		return nil, err
	}
	files := make([]File, count)
	for i := range files {
		path, err := r.path()
		if err != nil {
			return nil, err
		}
		size, err := r.signed()
		if err != nil {
			return nil, err
		}
		modTime, err := r.signed()
		if err != nil {
			return nil, err
		}
		hash, err := r.hash()
		if err != nil {
			return nil, err
		}
		files[i] = File{Path: path, Size: size, ModTime: modTime, SHA256: hash}
	}
	return files, nil
}

func (r *testCacheHeaderReader) directories() ([]Directory, error) {
	present, err := r.boolean()
	if err != nil || !present {
		return nil, err
	}
	count, err := r.count()
	if err != nil {
		return nil, err
	}
	directories := make([]Directory, count)
	for i := range directories {
		path, err := r.path()
		if err != nil {
			return nil, err
		}
		modTime, err := r.signed()
		if err != nil {
			return nil, err
		}
		directories[i] = Directory{Path: path, ModTime: modTime}
	}
	return directories, nil
}

func (r *testCacheHeaderReader) strings() ([]string, error) {
	present, err := r.boolean()
	if err != nil || !present {
		return nil, err
	}
	count, err := r.count()
	if err != nil {
		return nil, err
	}
	values := make([]string, count)
	for i := range values {
		value, err := r.string()
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func (r *testCacheHeaderReader) count() (int, error) {
	value, err := r.unsigned()
	if err != nil || value > maxTestCacheHeaderItems {
		return 0, errMalformedTestCacheHeader
	}
	if value > uint64(r.remaining()) {
		return 0, errMalformedTestCacheHeader
	}
	return int(value), nil
}

func (r *testCacheHeaderReader) path() (string, error) {
	return r.limitedString(maxTestCacheHeaderPathBytes)
}

func (r *testCacheHeaderReader) string() (string, error) {
	return r.limitedString(maxTestCacheHeaderStringBytes)
}

func (r *testCacheHeaderReader) limitedString(limit uint64) (string, error) {
	length, err := r.unsigned()
	if err != nil || length > limit || length > uint64(r.remaining()) {
		return "", errMalformedTestCacheHeader
	}
	data, err := r.take(int(length))
	if err != nil {
		return "", errMalformedTestCacheHeader
	}
	return string(data), nil
}

func (r *testCacheHeaderReader) hash() (string, error) {
	encoded, err := r.take(1)
	if err != nil {
		return "", errMalformedTestCacheHeader
	}
	kind := encoded[0]
	switch kind {
	case 0:
		return "", nil
	case 1:
		raw, err := r.take(sha256.Size)
		if err != nil {
			return "", errMalformedTestCacheHeader
		}
		return encodeLowerHex(raw), nil
	case 2:
		return r.string()
	default:
		return "", errMalformedTestCacheHeader
	}
}

func encodeLowerHex(raw []byte) string {
	const digits = "0123456789abcdef"
	var encoded [sha256HexLength]byte
	for i, value := range raw {
		encoded[i*2] = digits[value>>4]
		encoded[i*2+1] = digits[value&0x0f]
	}
	return string(encoded[:])
}

func (r *testCacheHeaderReader) boolean() (bool, error) {
	encoded, err := r.take(1)
	if err != nil {
		return false, errMalformedTestCacheHeader
	}
	value := encoded[0]
	if value > 1 {
		return false, errMalformedTestCacheHeader
	}
	return value == 1, nil
}

func (r *testCacheHeaderReader) signed() (int64, error) {
	value, n := binary.Varint(r.body[r.offset:])
	if n <= 0 {
		return 0, errMalformedTestCacheHeader
	}
	r.offset += n
	return value, nil
}

func (r *testCacheHeaderReader) unsigned() (uint64, error) {
	value, n := binary.Uvarint(r.body[r.offset:])
	if n <= 0 {
		return 0, errMalformedTestCacheHeader
	}
	r.offset += n
	return value, nil
}

func (r *testCacheHeaderReader) remaining() int {
	return len(r.body) - r.offset
}

func (r *testCacheHeaderReader) take(length int) ([]byte, error) {
	if length < 0 || length > r.remaining() {
		return nil, errMalformedTestCacheHeader
	}
	start := r.offset
	r.offset += length
	return r.body[start:r.offset], nil
}
