package convert

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type stTensorInfo struct {
	DType       string    `json:"dtype"`
	Shape       []int64   `json:"shape"`
	DataOffsets [2]uint64 `json:"data_offsets"`
}

type safeTensorsFile struct {
	path       string
	headerSize uint64
	tensors    map[string]stTensorInfo
}

// scanSafeTensorsFiles finds and parses all .safetensors files in a directory.
func scanSafeTensorsFiles(dir string) ([]*safeTensorsFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".safetensors") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no .safetensors files found in %s", dir)
	}

	sort.Strings(paths)

	var files []*safeTensorsFile
	for _, p := range paths {
		sf, err := parseSafeTensorsHeader(p)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filepath.Base(p), err)
		}
		files = append(files, sf)
	}

	return files, nil
}

func parseSafeTensorsHeader(path string) (*safeTensorsFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var headerLen uint64
	if err := binary.Read(f, binary.LittleEndian, &headerLen); err != nil {
		return nil, fmt.Errorf("reading header length: %w", err)
	}

	if headerLen > 100*1024*1024 {
		return nil, fmt.Errorf("header too large: %d bytes", headerLen)
	}

	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(f, headerBytes); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(headerBytes, &raw); err != nil {
		return nil, fmt.Errorf("parsing header JSON: %w", err)
	}

	tensors := make(map[string]stTensorInfo)
	for name, data := range raw {
		if name == "__metadata__" {
			continue
		}
		var info stTensorInfo
		if err := json.Unmarshal(data, &info); err != nil {
			return nil, fmt.Errorf("parsing tensor %q: %w", name, err)
		}
		tensors[name] = info
	}

	return &safeTensorsFile{
		path:       path,
		headerSize: headerLen,
		tensors:    tensors,
	}, nil
}

// dataOffset returns the byte offset where tensor data begins in the file.
func (sf *safeTensorsFile) dataOffset() uint64 {
	return 8 + sf.headerSize
}

// readTensorData reads raw bytes for a named tensor.
func (sf *safeTensorsFile) readTensorData(name string) ([]byte, error) {
	info, ok := sf.tensors[name]
	if !ok {
		return nil, fmt.Errorf("tensor %q not found in %s", name, filepath.Base(sf.path))
	}

	size := info.DataOffsets[1] - info.DataOffsets[0]
	if size == 0 {
		return nil, nil
	}

	f, err := os.Open(sf.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	offset := int64(sf.dataOffset() + info.DataOffsets[0])
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking to tensor data: %w", err)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, fmt.Errorf("reading tensor data: %w", err)
	}

	return data, nil
}

// stDTypeToGGML maps SafeTensors dtype strings to GGML types.
func stDTypeToGGML(dtype string) (GGMLType, error) {
	switch strings.ToUpper(dtype) {
	case "F16":
		return GGMLTypeF16, nil
	case "BF16":
		return GGMLTypeBF16, nil
	case "F32":
		return GGMLTypeF32, nil
	case "F64":
		return GGMLTypeF64, nil
	case "I8":
		return GGMLTypeI8, nil
	case "I16":
		return GGMLTypeI16, nil
	case "I32":
		return GGMLTypeI32, nil
	case "I64":
		return GGMLTypeI64, nil
	default:
		return 0, fmt.Errorf("unsupported SafeTensors dtype: %s", dtype)
	}
}

// collectTensors gathers all tensor metadata from multiple SafeTensors files.
func collectTensors(files []*safeTensorsFile) []tensorSource {
	var result []tensorSource
	for _, sf := range files {
		for name, info := range sf.tensors {
			result = append(result, tensorSource{
				name:  name,
				shape: info.Shape,
				dtype: info.DType,
				file:  sf,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result
}

type tensorSource struct {
	name  string
	shape []int64
	dtype string
	file  *safeTensorsFile
}

func (t *tensorSource) readData() ([]byte, error) {
	return t.file.readTensorData(t.name)
}
