package convert

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

const (
	ggufMagic     uint32 = 0x46554747 // "GGUF" in little-endian
	ggufVersion   uint32 = 3
	ggufAlignment uint64 = 32
)

// GGML tensor data types.
type GGMLType uint32

const (
	GGMLTypeF32  GGMLType = 0
	GGMLTypeF16  GGMLType = 1
	GGMLTypeI8   GGMLType = 24
	GGMLTypeI16  GGMLType = 25
	GGMLTypeI32  GGMLType = 26
	GGMLTypeI64  GGMLType = 27
	GGMLTypeF64  GGMLType = 28
	GGMLTypeBF16 GGMLType = 30
)

// Bytes per element for each GGML type.
func (t GGMLType) ElementSize() uint64 {
	switch t {
	case GGMLTypeF32, GGMLTypeI32:
		return 4
	case GGMLTypeF16, GGMLTypeBF16, GGMLTypeI16:
		return 2
	case GGMLTypeI8:
		return 1
	case GGMLTypeI64, GGMLTypeF64:
		return 8
	default:
		return 0
	}
}

// GGUF metadata value types.
const (
	ggufValUint8   uint32 = 0
	ggufValInt8    uint32 = 1
	ggufValUint16  uint32 = 2
	ggufValInt16   uint32 = 3
	ggufValUint32  uint32 = 4
	ggufValInt32   uint32 = 5
	ggufValFloat32 uint32 = 6
	ggufValBool    uint32 = 7
	ggufValString  uint32 = 8
	ggufValArray   uint32 = 9
	ggufValUint64  uint32 = 10
	ggufValInt64   uint32 = 11
	ggufValFloat64 uint32 = 12
)

type ggufKV struct {
	key   string
	value interface{}
}

type ggufTensor struct {
	name    string
	dims    []uint64 // GGML dimension order (reversed from HF/PyTorch)
	dtype   GGMLType
	size    uint64
	offset  uint64 // offset within data section
	getData func() ([]byte, error)
}

type ggufWriter struct {
	kvs     []ggufKV
	tensors []ggufTensor
}

func newGGUFWriter() *ggufWriter {
	return &ggufWriter{}
}

func (w *ggufWriter) addKV(key string, value interface{}) {
	w.kvs = append(w.kvs, ggufKV{key: key, value: value})
}

func (w *ggufWriter) addTensor(name string, dims []uint64, dtype GGMLType, getData func() ([]byte, error)) {
	numElements := uint64(1)
	for _, d := range dims {
		numElements *= d
	}
	size := numElements * dtype.ElementSize()

	w.tensors = append(w.tensors, ggufTensor{
		name:    name,
		dims:    dims,
		dtype:   dtype,
		size:    size,
		getData: getData,
	})
}

func (w *ggufWriter) writeTo(path string) error {
	// Calculate tensor offsets in data section.
	var dataSize uint64
	for i := range w.tensors {
		w.tensors[i].offset = dataSize
		dataSize += w.tensors[i].size
		if i < len(w.tensors)-1 {
			dataSize = alignUp(dataSize, ggufAlignment)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating GGUF file: %w", err)
	}
	defer f.Close()

	bw := &binaryWriter{w: f}

	// Header.
	bw.u32(ggufMagic)
	bw.u32(ggufVersion)
	bw.u64(uint64(len(w.tensors)))
	bw.u64(uint64(len(w.kvs)))

	// Metadata KV pairs.
	for _, kv := range w.kvs {
		bw.ggufString(kv.key)
		if err := bw.ggufValue(kv.value); err != nil {
			return fmt.Errorf("writing KV %q: %w", kv.key, err)
		}
	}

	// Tensor info.
	for _, t := range w.tensors {
		bw.ggufString(t.name)
		bw.u32(uint32(len(t.dims)))
		for _, d := range t.dims {
			bw.u64(d)
		}
		bw.u32(uint32(t.dtype))
		bw.u64(t.offset)
	}

	if bw.err != nil {
		return fmt.Errorf("writing header: %w", bw.err)
	}

	// Pad to alignment boundary for data section.
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("getting position: %w", err)
	}
	padBytes := alignUp(uint64(pos), ggufAlignment) - uint64(pos)
	if padBytes > 0 {
		if _, err := f.Write(make([]byte, padBytes)); err != nil {
			return fmt.Errorf("writing alignment padding: %w", err)
		}
	}

	// Tensor data.
	for i, t := range w.tensors {
		data, err := t.getData()
		if err != nil {
			return fmt.Errorf("reading tensor %q: %w", t.name, err)
		}
		if uint64(len(data)) != t.size {
			return fmt.Errorf("tensor %q: expected %d bytes, got %d", t.name, t.size, len(data))
		}
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("writing tensor %q: %w", t.name, err)
		}

		// Pad between tensors.
		if i < len(w.tensors)-1 {
			pad := alignUp(t.size, ggufAlignment) - t.size
			if pad > 0 {
				if _, err := f.Write(make([]byte, pad)); err != nil {
					return fmt.Errorf("writing tensor padding: %w", err)
				}
			}
		}
	}

	return f.Close()
}

func alignUp(v, alignment uint64) uint64 {
	return (v + alignment - 1) & ^(alignment - 1)
}

// binaryWriter wraps an io.Writer with error tracking.
type binaryWriter struct {
	w   io.Writer
	err error
}

func (bw *binaryWriter) write(data interface{}) {
	if bw.err != nil {
		return
	}
	bw.err = binary.Write(bw.w, binary.LittleEndian, data)
}

func (bw *binaryWriter) u8(v uint8)   { bw.write(v) }
func (bw *binaryWriter) u16(v uint16) { bw.write(v) }
func (bw *binaryWriter) u32(v uint32) { bw.write(v) }
func (bw *binaryWriter) u64(v uint64) { bw.write(v) }
func (bw *binaryWriter) f32(v float32) { bw.write(v) }
func (bw *binaryWriter) f64(v float64) { bw.write(v) }

func (bw *binaryWriter) bytes(data []byte) {
	if bw.err != nil {
		return
	}
	_, bw.err = bw.w.Write(data)
}

// ggufString writes a GGUF-format string: uint64(len) + bytes (no null terminator).
func (bw *binaryWriter) ggufString(s string) {
	bw.u64(uint64(len(s)))
	bw.bytes([]byte(s))
}

// ggufValue writes a typed GGUF metadata value.
func (bw *binaryWriter) ggufValue(v interface{}) error {
	switch val := v.(type) {
	case string:
		bw.u32(ggufValString)
		bw.ggufString(val)
	case uint32:
		bw.u32(ggufValUint32)
		bw.u32(val)
	case int32:
		bw.u32(ggufValInt32)
		bw.write(val)
	case uint64:
		bw.u32(ggufValUint64)
		bw.u64(val)
	case int64:
		bw.u32(ggufValInt64)
		bw.write(val)
	case float32:
		bw.u32(ggufValFloat32)
		bw.f32(val)
	case float64:
		bw.u32(ggufValFloat64)
		bw.f64(val)
	case bool:
		bw.u32(ggufValBool)
		if val {
			bw.u8(1)
		} else {
			bw.u8(0)
		}
	case []string:
		bw.u32(ggufValArray)
		bw.u32(ggufValString)
		bw.u64(uint64(len(val)))
		for _, s := range val {
			bw.ggufString(s)
		}
	case []float32:
		bw.u32(ggufValArray)
		bw.u32(ggufValFloat32)
		bw.u64(uint64(len(val)))
		for _, f := range val {
			bw.f32(f)
		}
	case []int32:
		bw.u32(ggufValArray)
		bw.u32(ggufValInt32)
		bw.u64(uint64(len(val)))
		for _, i := range val {
			bw.write(i)
		}
	case []uint32:
		bw.u32(ggufValArray)
		bw.u32(ggufValUint32)
		bw.u64(uint64(len(val)))
		for _, i := range val {
			bw.u32(i)
		}
	default:
		return fmt.Errorf("unsupported GGUF value type: %T", v)
	}
	return bw.err
}

// bf16ToF32 converts bfloat16 raw bytes to float32 raw bytes.
func bf16ToF32(data []byte) []byte {
	if len(data)%2 != 0 {
		return data
	}
	out := make([]byte, len(data)*2)
	for i := 0; i < len(data); i += 2 {
		bf16 := uint16(data[i]) | uint16(data[i+1])<<8
		f32bits := uint32(bf16) << 16
		j := i * 2
		out[j] = byte(f32bits)
		out[j+1] = byte(f32bits >> 8)
		out[j+2] = byte(f32bits >> 16)
		out[j+3] = byte(f32bits >> 24)
	}
	return out
}

// bf16ToF16 converts bfloat16 raw bytes to float16 raw bytes.
// bfloat16: 1 sign + 8 exponent + 7 mantissa (same range as float32)
// float16:  1 sign + 5 exponent + 10 mantissa (limited range)
func bf16ToF16(data []byte) []byte {
	if len(data)%2 != 0 {
		return data
	}
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += 2 {
		bf16 := uint16(data[i]) | uint16(data[i+1])<<8
		f32bits := uint32(bf16) << 16
		f16 := float32BitsToFloat16(f32bits)
		out[i] = byte(f16)
		out[i+1] = byte(f16 >> 8)
	}
	return out
}

// f16ToF32 converts little-endian float16 raw bytes to float32 raw bytes.
func f16ToF32(data []byte) []byte {
	if len(data)%2 != 0 {
		return data
	}
	out := make([]byte, len(data)*2)
	for i := 0; i < len(data); i += 2 {
		h := binary.LittleEndian.Uint16(data[i:])
		binary.LittleEndian.PutUint32(out[i*2:], math.Float32bits(float16ToFloat32(h)))
	}
	return out
}

func float32BitsToFloat16(bits uint32) uint16 {
	f := math.Float32frombits(bits)
	if math.IsNaN(float64(f)) {
		return 0x7E00
	}
	if math.IsInf(float64(f), 1) {
		return 0x7C00
	}
	if math.IsInf(float64(f), -1) {
		return 0xFC00
	}

	sign := uint16((bits >> 16) & 0x8000)
	exp := int((bits>>23)&0xFF) - 127
	mantissa := bits & 0x7FFFFF

	if exp > 15 {
		return sign | 0x7C00 // overflow → infinity
	}
	if exp < -24 {
		return sign // underflow → zero
	}
	if exp < -14 {
		// Subnormal float16.
		mantissa |= 0x800000
		shift := uint(-14 - exp)
		mantissa >>= shift
		return sign | uint16(mantissa>>13)
	}

	return sign | uint16(exp+15)<<10 | uint16(mantissa>>13)
}
