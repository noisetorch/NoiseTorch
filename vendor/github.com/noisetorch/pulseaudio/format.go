package pulseaudio

import (
	"encoding/binary"
	"fmt"
	"io"
)

type tagType byte

const (
	invalidTag    tagType = 0
	stringTag     tagType = 't'
	stringNullTag tagType = 'N'
	uint32Tag     tagType = 'L'
	uint8Tag      tagType = 'B'
	uint64Tag     tagType = 'R'
	int64Tag      tagType = 'r'
	sampleSpecTag tagType = 'a'
	arbitraryTag  tagType = 'x'
	trueTag       tagType = '1'
	falseTag      tagType = '0'
	timeTag       tagType = 'T'
	usecTag       tagType = 'U'
	channelMapTag tagType = 'm'
	cvolumeTag    tagType = 'v'
	propListTag   tagType = 'P'
	volumeTag     tagType = 'V'
	formatInfoTag tagType = 'f'
)

func (t tagType) String() string {
	switch t {
	case invalidTag:
		return "invalidTag"
	case stringTag:
		return "stringTag"
	case stringNullTag:
		return "stringNullTag"
	case uint32Tag:
		return "uint32Tag"
	case uint8Tag:
		return "uint8Tag"
	case uint64Tag:
		return "uint64Tag"
	case int64Tag:
		return "int64Tag"
	case sampleSpecTag:
		return "sampleSpecTag"
	case arbitraryTag:
		return "arbitraryTag"
	case trueTag:
		return "trueTag"
	case falseTag:
		return "falseTag"
	case timeTag:
		return "timeTag"
	case usecTag:
		return "usecTag"
	case channelMapTag:
		return "channelMapTag"
	case cvolumeTag:
		return "cvolumeTag"
	case propListTag:
		return "propListTag"
	case volumeTag:
		return "volumeTag"
	case formatInfoTag:
		return "formatInfoTag"
	default:
		return fmt.Sprintf("UnknownValue(%d)", t)
	}
}

type binaryReader interface {
	readFrom(r io.Reader, c *Client) error
}

func bwrite(w io.Writer, data ...interface{}) error {
	for _, v := range data {
		if propList, ok := v.(map[string]string); ok {
			err := bwrite(w, propListTag)
			if err != nil {
				return err
			}
			for k, v := range propList {
				if v == "" {
					continue
				}

				l := uint32(len(v) + 1) // +1 for null at the end of string
				err := bwrite(w,
					stringTag, []byte(k), byte(0),
					uint32Tag, l,
					arbitraryTag, l,
					[]byte(v), byte(0),
				)
				if err != nil {
					return err
				}
			}
			err = bwrite(w, stringNullTag)
			if err != nil {
				return err
			}
			continue
		}

		if cvolume, ok := v.(cvolume); ok {
			arr := []uint32(cvolume)
			err := bwrite(w, cvolumeTag, byte(len(arr)), arr)
			if err != nil {
				return err
			}
			continue
		}

		if err := binary.Write(w, binary.BigEndian, v); err != nil {
			return err
		}
	}
	return nil
}

func bread(r io.Reader, data ...interface{}) error {

	nullString := false

	for _, v := range data {

		if nullString {
			nullString = false
			continue
		}

		t, ok := v.(tagType)
		if ok {
			var tt tagType
			if err := binary.Read(r, binary.BigEndian, &tt); err != nil {
				return err
			}

			// if we get a null string, we want to skip the next data reading cycle, as the string will be initialized as empty
			// and we want to exit this cycle, as we now know it's a null string. That's why we have the weird bool flag, to
			// essentially "continue" twice.
			if tt == stringNullTag {
				nullString = true
				continue
			}

			if tt != t {
				return fmt.Errorf("Protcol error: Got type %s but expected %s", tt, t)
			}
			continue
		}

		sptr, ok := v.(*string)
		if ok {
			buf := make([]byte, 0)
			i := 0
			for {
				var curChar [1]byte
				_, err := r.Read(curChar[:])
				if err != nil {
					return err
				}
				buf = append(buf, curChar[0])
				if buf[i] == 0 {
					*sptr = string(buf[:i])
					break
				} else {
					if i > len(buf) {
						return fmt.Errorf("String is too long (max %d bytes)", len(buf))
					}
					i++
				}
			}
			continue
		}

		propList, ok := v.(*map[string]string)
		if ok {
			*propList = make(map[string]string)
			err := bread(r, propListTag)
			if err != nil {
				return err
			}
			for {
				var t tagType
				if err = bread(r, &t); err != nil {
					return err
				}
				if t == stringNullTag {
					// end of the proplist.
					break
				}
				if t != stringTag {
					return fmt.Errorf("Protcol error: Got type %s but expected %s", t, stringTag)
				}

				var k, v string
				var l1, l2 uint32
				if err = bread(r,
					&k,
					uint32Tag, &l1,
					arbitraryTag, &l2,
					&v,
				); err != nil {
					return err
				}
				if len(v) != int(l1-1) || len(v) != int(l2-1) {
					return fmt.Errorf("Protocol error: Proplist value length mismatch (len %d, arb len %d, value len %d)",
						l1, l2, len(v))
				}
				(*propList)[k] = v
			}
			continue
		}

		rdr, ok := v.(io.ReaderFrom)
		if ok {
			if _, err := rdr.ReadFrom(r); err != nil {
				return err
			}
			continue
		}

		bptr, ok := v.(*bool)
		if ok {
			var tt tagType
			if err := binary.Read(r, binary.BigEndian, &tt); err != nil {
				return err
			}
			if tt == trueTag {
				*bptr = true
			} else if tt == falseTag {
				*bptr = false
			} else {
				return fmt.Errorf("Protcol error: Got type %s but expected boolean true or false", tt)
			}
			continue
		}

		if err := binary.Read(r, binary.BigEndian, v); err != nil {
			return err
		}
	}
	return nil
}
