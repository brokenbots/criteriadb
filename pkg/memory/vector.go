package memory

import (
	"encoding/binary"
	"math"
)

// PackFloats encodes []float32 into little-endian byte slice for compact Protobuf packing.
func PackFloats(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// UnpackFloats decodes little-endian byte slice back to []float32.
func UnpackFloats(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := 0; i < len(v); i++ {
		bits := binary.LittleEndian.Uint32(b[i*4:])
		v[i] = math.Float32frombits(bits)
	}
	return v
}

// CosineSimilarity computes cosine similarity between two float32 slices.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		valA := float64(a[i])
		valB := float64(b[i])
		dotProduct += valA * valB
		normA += valA * valA
		normB += valB * valB
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0.0 || math.IsNaN(denom) {
		return 0.0
	}

	sim := dotProduct / denom
	if math.IsNaN(sim) || math.IsInf(sim, 0) {
		return 0.0
	}
	// Clamp to [0.0, 1.0] for similarity score normalization
	if sim < 0.0 {
		return 0.0
	}
	if sim > 1.0 {
		return 1.0
	}
	return sim
}
