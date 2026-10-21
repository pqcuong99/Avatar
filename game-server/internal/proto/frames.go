// Package proto encodes and decodes the binary realtime frames described in
// shared/proto/realtime.proto, using the protobuf wire format directly.
package proto

import (
	"errors"

	"google.golang.org/protobuf/encoding/protowire"
)

type MoveInput struct {
	Seq    uint32
	DX, DY int32
}

type EntityState struct {
	EntityID uint32 `json:"entity_id"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
	Dir      uint32 `json:"dir"`
	Moving   bool   `json:"moving"`
}

type Snapshot struct {
	ServerTick uint32
	AckSeq     uint32
	Entities   []EntityState
}

var ErrMalformed = errors.New("proto: malformed frame")

// DecodeClientFrame returns the MoveInput of a ClientFrame. Unknown fields
// are skipped, as protobuf requires.
func DecodeClientFrame(b []byte) (MoveInput, bool, error) {
	var in MoveInput
	found := false
	err := eachField(b, func(num protowire.Number, typ protowire.Type, v []byte, _ uint64) error {
		if num == 1 && typ == protowire.BytesType {
			found = true
			return eachField(v, func(num protowire.Number, typ protowire.Type, _ []byte, n uint64) error {
				if typ != protowire.VarintType {
					return nil
				}
				switch num {
				case 1:
					in.Seq = uint32(n)
				case 2:
					in.DX = int32(protowire.DecodeZigZag(n))
				case 3:
					in.DY = int32(protowire.DecodeZigZag(n))
				}
				return nil
			})
		}
		return nil
	})
	return in, found, err
}

// EncodeClientFrame is used by tests and bots.
func EncodeClientFrame(in MoveInput) []byte {
	var m []byte
	m = appendUint(m, 1, uint64(in.Seq))
	m = appendSint(m, 2, in.DX)
	m = appendSint(m, 3, in.DY)
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	return protowire.AppendBytes(b, m)
}

// EncodeEntities encodes the repeated `entities` field (number 3) of a
// WorldSnapshot once per room tick; EncodeSnapshot adds the per-receiver
// fields around it. Protobuf allows fields in any order.
func EncodeEntities(es []EntityState) []byte {
	var b []byte
	for _, e := range es {
		var m []byte
		m = appendUint(m, 1, uint64(e.EntityID))
		m = appendSint(m, 2, e.X)
		m = appendSint(m, 3, e.Y)
		m = appendUint(m, 4, uint64(e.Dir))
		if e.Moving {
			m = appendUint(m, 5, 1)
		}
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, m)
	}
	return b
}

// EncodeSnapshot builds a ServerFrame{snapshot}.
func EncodeSnapshot(serverTick, ackSeq uint32, entities []byte) []byte {
	var s []byte
	s = appendUint(s, 1, uint64(serverTick))
	s = appendUint(s, 2, uint64(ackSeq))
	s = append(s, entities...)
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	return protowire.AppendBytes(b, s)
}

// DecodeSnapshot is used by tests and bots.
func DecodeSnapshot(b []byte) (Snapshot, error) {
	var s Snapshot
	err := eachField(b, func(num protowire.Number, typ protowire.Type, v []byte, _ uint64) error {
		if num != 1 || typ != protowire.BytesType {
			return nil
		}
		return eachField(v, func(num protowire.Number, typ protowire.Type, v []byte, n uint64) error {
			switch {
			case num == 1 && typ == protowire.VarintType:
				s.ServerTick = uint32(n)
			case num == 2 && typ == protowire.VarintType:
				s.AckSeq = uint32(n)
			case num == 3 && typ == protowire.BytesType:
				var e EntityState
				err := eachField(v, func(num protowire.Number, typ protowire.Type, _ []byte, n uint64) error {
					if typ != protowire.VarintType {
						return nil
					}
					switch num {
					case 1:
						e.EntityID = uint32(n)
					case 2:
						e.X = int32(protowire.DecodeZigZag(n))
					case 3:
						e.Y = int32(protowire.DecodeZigZag(n))
					case 4:
						e.Dir = uint32(n)
					case 5:
						e.Moving = n != 0
					}
					return nil
				})
				s.Entities = append(s.Entities, e)
				return err
			}
			return nil
		})
	})
	return s, err
}

func appendUint(b []byte, num protowire.Number, v uint64) []byte {
	if v == 0 {
		return b // proto3 default values are not encoded
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

func appendSint(b []byte, num protowire.Number, v int32) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, protowire.EncodeZigZag(int64(v)))
}

// eachField walks the fields of one message. For varints n holds the value,
// for length-delimited fields v holds the bytes.
func eachField(b []byte, fn func(num protowire.Number, typ protowire.Type, v []byte, n uint64) error) error {
	for len(b) > 0 {
		num, typ, l := protowire.ConsumeTag(b)
		if l < 0 {
			return ErrMalformed
		}
		b = b[l:]
		var v []byte
		var n uint64
		switch typ {
		case protowire.VarintType:
			n, l = protowire.ConsumeVarint(b)
		case protowire.BytesType:
			v, l = protowire.ConsumeBytes(b)
		default:
			l = protowire.ConsumeFieldValue(num, typ, b)
		}
		if l < 0 {
			return ErrMalformed
		}
		b = b[l:]
		if err := fn(num, typ, v, n); err != nil {
			return err
		}
	}
	return nil
}
