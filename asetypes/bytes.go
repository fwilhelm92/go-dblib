// SPDX-FileCopyrightText: 2020 SAP SE
// SPDX-FileCopyrightText: 2021 SAP SE
//
// SPDX-License-Identifier: Apache-2.0

package asetypes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
	"unicode/utf16"

	"github.com/SAP/go-dblib/asetime"
)

// Bytes returns a byte slice based on a given value-interface and depending
// on the ASE data type.
func (t DataType) Bytes(endian binary.ByteOrder, value interface{}) ([]byte, error) {
	switch t {
	case MONEY, SHORTMONEY:
		bs := make([]byte, t.ByteSize())
		dec, ok := value.(*Decimal)
		if !ok {
			return nil, fmt.Errorf("expected *asetypes.Decimal for %s, received %T", t, value)
		}
		deci := dec.Int()

		if t == MONEY {
			endian.PutUint32(bs[:4], uint32(deci.Int64()>>32))
			endian.PutUint32(bs[4:], uint32(deci.Int64()))
		} else {
			endian.PutUint32(bs, uint32(deci.Int64()))
		}

		return bs, nil
	case DECN, NUMN:
		dec, ok := value.(*Decimal)
		if !ok {
			return nil, fmt.Errorf("expected *asetypes.Decimal for %s, received %T", t, value)
		}

		bs := make([]byte, dec.ByteSize())
		copy(bs[dec.ByteSize()-len(dec.Bytes()):], dec.Bytes())
		if dec.IsNegative() {
			bs[0] = 0x1
		}
		return bs, nil
	case DATE, DATEN:
		t := asetime.DurationFromDateTime(value.(time.Time))
		t -= asetime.DurationFromDateTime(asetime.Epoch1900())

		bs := make([]byte, 4)
		endian.PutUint32(bs, uint32(t.Days()))
		return bs, nil
	case TIME, TIMEN:
		dur := asetime.DurationFromTime(value.(time.Time))
		fract := asetime.MillisecondToFractionalSecond(dur.Microseconds())

		bs := make([]byte, 4)
		endian.PutUint32(bs, uint32(fract))
		return bs, nil
	case SHORTDATE:
		t := asetime.DurationFromDateTime(value.(time.Time))
		t -= asetime.DurationFromDateTime(asetime.Epoch1900())

		days := t.Days()
		s := asetime.ASEDuration(t.Microseconds() - days*int(asetime.Day))

		bs := make([]byte, 4)
		// TODO replace all binary.Littleendian
		binary.LittleEndian.PutUint16(bs[:2], uint16(days))
		binary.LittleEndian.PutUint16(bs[2:], uint16(s.Minutes()))
		return bs, nil
	case DATETIME:
		t := asetime.DurationFromDateTime(value.(time.Time))
		t -= asetime.DurationFromDateTime(asetime.Epoch1900())

		days := t.Days()
		s := t.Microseconds() - days*int(asetime.Day)
		s = asetime.MillisecondToFractionalSecond(s)

		bs := make([]byte, 8)
		binary.LittleEndian.PutUint32(bs[:4], uint32(days))
		binary.LittleEndian.PutUint32(bs[4:], uint32(s))
		return bs, nil
	case BIGDATETIMEN:
		dur := asetime.DurationFromDateTime(value.(time.Time))

		bs := make([]byte, 8)
		binary.LittleEndian.PutUint64(bs, uint64(dur))
		return bs, nil
	case BIGTIMEN:
		dur := asetime.DurationFromTime(value.(time.Time))

		bs := make([]byte, 8)
		binary.LittleEndian.PutUint64(bs, uint64(dur))
		return bs, nil
	case UNITEXT:
		// convert go string to utf16 code points
		runes := []rune(value.(string))
		utf16bytes := utf16.Encode(runes)

		// convert utf16 code points to bytes
		bs := make([]byte, len(utf16bytes)*2)
		for i := 0; i < len(utf16bytes); i++ {
			binary.LittleEndian.PutUint16(bs[i:], utf16bytes[i])
		}

		return bs, nil
	}

	switch typed := value.(type) {
	case string:
		value = []byte(typed)
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, endian, value); err != nil {
		return nil, fmt.Errorf("error writing value: %w", err)
	}

	bs := buf.Bytes()
	if t.ByteSize() != -1 && t.ByteSize() != len(bs) {
		return nil, fmt.Errorf("binary.Write returned a byteslice of length %d, expected %d for datatype %s",
			len(bs), t.ByteSize(), t)
	}

	return bs, nil
}
