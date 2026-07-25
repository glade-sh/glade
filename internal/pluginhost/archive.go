package pluginhost

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type extractedArchive struct {
	dir   string
	files map[string]extractedFile
}

type extractedFile struct {
	path string
	mode os.FileMode
}

func extractArchive(archivePath, parent string) (extractedArchive, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return extractedArchive{}, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return extractedArchive{}, err
	}
	defer gz.Close()
	tmp, err := os.MkdirTemp(parent, ".extract-*")
	if err != nil {
		return extractedArchive{}, err
	}
	root, err := os.OpenRoot(tmp)
	if err != nil {
		os.RemoveAll(tmp)
		return extractedArchive{}, err
	}
	defer root.Close()
	out := extractedArchive{dir: tmp, files: map[string]extractedFile{}}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(tmp)
		}
	}()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return extractedArchive{}, err
		}
		name, err := safeArchivePath(header.Name)
		if err != nil {
			return extractedArchive{}, err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, os.FileMode(header.Mode&0o777)); err != nil {
				return extractedArchive{}, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if parent := path.Dir(name); parent != "." {
				if err := root.MkdirAll(parent, 0o755); err != nil {
					return extractedArchive{}, err
				}
			}
			mode := os.FileMode(header.Mode & 0o777)
			if mode == 0 {
				mode = 0o644
			}
			if err := writeArchiveFile(root, name, tr, mode); err != nil {
				return extractedArchive{}, err
			}
			out.files[name] = extractedFile{path: filepath.Join(tmp, filepath.FromSlash(name)), mode: mode}
		default:
			return extractedArchive{}, fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
	cleanup = false
	return out, nil
}

func writeArchiveFile(root *os.Root, name string, r io.Reader, mode os.FileMode) error {
	out, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return root.Chmod(name, mode)
}

func safeArchivePath(name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.Contains(name, `\`) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return clean, nil
}

func readChecksums(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	checksums := map[string]string{}
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksums.txt line %d", lineNo+1)
		}
		if len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid sha256 on checksums.txt line %d", lineNo+1)
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return nil, fmt.Errorf("invalid sha256 on checksums.txt line %d", lineNo+1)
		}
		name, err := safeArchivePath(fields[1])
		if err != nil {
			return nil, err
		}
		checksums[name] = strings.ToLower(fields[0])
	}
	if len(checksums) == 0 {
		return nil, errors.New("checksums.txt is empty")
	}
	return checksums, nil
}

func verifyArchiveChecksums(dir string, files map[string]extractedFile, checksums map[string]string) error {
	for name, want := range checksums {
		path := filepath.Join(dir, filepath.FromSlash(name))
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		if sum != want {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
	}
	for name := range files {
		if name == "checksums.txt" {
			continue
		}
		if _, ok := checksums[name]; !ok {
			return fmt.Errorf("archive file %s is missing from checksums.txt", name)
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
