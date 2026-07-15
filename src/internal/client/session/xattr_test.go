package session

import (
	"testing"
	"wsfs-core/internal/share/wsfsprotocol"
	"wsfs-core/internal/util"
)

func TestReadXAttrDataCombinesPartialResponses(t *testing.T) {
	s, err := NewSession(nil, 0, []string{"user.wsfs_test."}, true)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		s.responses[7] <- xattrResponse(7, wsfsprotocol.ErrorPartialResponse, []byte("first"))
		s.responses[7] <- xattrResponse(7, wsfsprotocol.ErrorOK, []byte("second"))
	}()

	data, code := s.readXAttrData(7)
	if code != wsfsprotocol.ErrorOK {
		t.Fatalf("code = %d, want OK", code)
	}
	if string(data) != "firstsecond" {
		t.Fatalf("data = %q, want %q", data, "firstsecond")
	}
}

func TestReadXAttrDataDiscardsPartialDataAfterFailure(t *testing.T) {
	s, err := NewSession(nil, 0, []string{"user.wsfs_test."}, true)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		s.responses[9] <- xattrResponse(9, wsfsprotocol.ErrorPartialResponse, []byte("partial"))
		s.responses[9] <- xattrResponse(9, wsfsprotocol.ErrorNoXAttr, []byte("ignored"))
	}()

	data, code := s.readXAttrData(9)
	if data != nil {
		t.Fatalf("data = %q, want nil", data)
	}
	if code != wsfsprotocol.ErrorNoXAttr {
		t.Fatalf("code = %d, want ErrorNoXAttr", code)
	}
}

func TestXAttrValidationAndChunkSizing(t *testing.T) {
	s, err := NewSession(nil, 0, []string{"user.wsfs_test."}, true)
	if err != nil {
		t.Fatal(err)
	}
	if code := s.validateXAttr("/file", "user.wsfs_test.key", 0, true); code != wsfsprotocol.ErrorOK {
		t.Fatalf("allowed key code = %d, want OK", code)
	}
	if code := s.validateXAttr("/file", "user.other.key", 0, true); code != wsfsprotocol.ErrorStateBlocked {
		t.Fatalf("blocked key code = %d, want ErrorStateBlocked", code)
	}
	if code := s.validateXAttr("/file", "user.wsfs_test.key", 1, false); code != wsfsprotocol.ErrorInvalid {
		t.Fatalf("get mode code = %d, want ErrorInvalid", code)
	}

	chunkSize := s.maxXAttrChunkSize("/file", "user.wsfs_test.key", 0)
	if chunkSize == 0 {
		t.Fatal("chunk size is zero")
	}
	if !s.setXAttrFits("/file", "user.wsfs_test.key", make([]byte, chunkSize), 0) {
		t.Fatal("maximum chunk does not fit")
	}
	if s.setXAttrFits("/file", "user.wsfs_test.key", make([]byte, chunkSize+1), 0) {
		t.Fatal("chunk larger than maximum fits")
	}
}

func xattrResponse(mark, code uint8, data []byte) *util.Buffer {
	buf := util.NewBuffer(maxFrameSize)
	buf.Write([]byte{mark, code})
	buf.Write(data)
	return buf
}
