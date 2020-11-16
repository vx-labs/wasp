// Code generated by protoc-gen-go. DO NOT EDIT.
// source: types.proto

package crdt

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type MockedEntry struct {
	ID                   string   `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	LastAdded            int64    `protobuf:"varint,2,opt,name=LastAdded,proto3" json:"LastAdded,omitempty"`
	LastDeleted          int64    `protobuf:"varint,3,opt,name=LastDeleted,proto3" json:"LastDeleted,omitempty"`
	Data                 string   `protobuf:"bytes,4,opt,name=Data,proto3" json:"Data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *MockedEntry) Reset()         { *m = MockedEntry{} }
func (m *MockedEntry) String() string { return proto.CompactTextString(m) }
func (*MockedEntry) ProtoMessage()    {}
func (*MockedEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_d938547f84707355, []int{0}
}

func (m *MockedEntry) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MockedEntry.Unmarshal(m, b)
}
func (m *MockedEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MockedEntry.Marshal(b, m, deterministic)
}
func (m *MockedEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MockedEntry.Merge(m, src)
}
func (m *MockedEntry) XXX_Size() int {
	return xxx_messageInfo_MockedEntry.Size(m)
}
func (m *MockedEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_MockedEntry.DiscardUnknown(m)
}

var xxx_messageInfo_MockedEntry proto.InternalMessageInfo

func (m *MockedEntry) GetID() string {
	if m != nil {
		return m.ID
	}
	return ""
}

func (m *MockedEntry) GetLastAdded() int64 {
	if m != nil {
		return m.LastAdded
	}
	return 0
}

func (m *MockedEntry) GetLastDeleted() int64 {
	if m != nil {
		return m.LastDeleted
	}
	return 0
}

func (m *MockedEntry) GetData() string {
	if m != nil {
		return m.Data
	}
	return ""
}

func init() {
	proto.RegisterType((*MockedEntry)(nil), "crdt.MockedEntry")
}

func init() { proto.RegisterFile("types.proto", fileDescriptor_d938547f84707355) }

var fileDescriptor_d938547f84707355 = []byte{
	// 134 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2e, 0xa9, 0x2c, 0x48,
	0x2d, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x49, 0x2e, 0x4a, 0x29, 0x51, 0x2a, 0xe4,
	0xe2, 0xf6, 0xcd, 0x4f, 0xce, 0x4e, 0x4d, 0x71, 0xcd, 0x2b, 0x29, 0xaa, 0x14, 0xe2, 0xe3, 0x62,
	0xf2, 0x74, 0x91, 0x60, 0x54, 0x60, 0xd4, 0xe0, 0x0c, 0x62, 0xf2, 0x74, 0x11, 0x92, 0xe1, 0xe2,
	0xf4, 0x49, 0x2c, 0x2e, 0x71, 0x4c, 0x49, 0x49, 0x4d, 0x91, 0x60, 0x52, 0x60, 0xd4, 0x60, 0x0e,
	0x42, 0x08, 0x08, 0x29, 0x70, 0x71, 0x83, 0x38, 0x2e, 0xa9, 0x39, 0xa9, 0x25, 0xa9, 0x29, 0x12,
	0xcc, 0x60, 0x79, 0x64, 0x21, 0x21, 0x21, 0x2e, 0x16, 0x97, 0xc4, 0x92, 0x44, 0x09, 0x16, 0xb0,
	0x89, 0x60, 0x76, 0x12, 0x1b, 0xd8, 0x7e, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff, 0xff, 0x5a, 0x87,
	0x6e, 0xb0, 0x8e, 0x00, 0x00, 0x00,
}