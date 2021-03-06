// Code generated by protoc-gen-go. DO NOT EDIT.
// source: topics.proto

package topics

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

type Node struct {
	Buf                  []byte           `protobuf:"bytes,2,opt,name=Buf,proto3" json:"Buf,omitempty"`
	Children             map[string]*Node `protobuf:"bytes,3,rep,name=Children,proto3" json:"Children,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}         `json:"-"`
	XXX_unrecognized     []byte           `json:"-"`
	XXX_sizecache        int32            `json:"-"`
}

func (m *Node) Reset()         { *m = Node{} }
func (m *Node) String() string { return proto.CompactTextString(m) }
func (*Node) ProtoMessage()    {}
func (*Node) Descriptor() ([]byte, []int) {
	return fileDescriptor_72af2b0eeeefc419, []int{0}
}

func (m *Node) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Node.Unmarshal(m, b)
}
func (m *Node) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Node.Marshal(b, m, deterministic)
}
func (m *Node) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Node.Merge(m, src)
}
func (m *Node) XXX_Size() int {
	return xxx_messageInfo_Node.Size(m)
}
func (m *Node) XXX_DiscardUnknown() {
	xxx_messageInfo_Node.DiscardUnknown(m)
}

var xxx_messageInfo_Node proto.InternalMessageInfo

func (m *Node) GetBuf() []byte {
	if m != nil {
		return m.Buf
	}
	return nil
}

func (m *Node) GetChildren() map[string]*Node {
	if m != nil {
		return m.Children
	}
	return nil
}

func init() {
	proto.RegisterType((*Node)(nil), "topics.Node")
	proto.RegisterMapType((map[string]*Node)(nil), "topics.Node.ChildrenEntry")
}

func init() { proto.RegisterFile("topics.proto", fileDescriptor_72af2b0eeeefc419) }

var fileDescriptor_72af2b0eeeefc419 = []byte{
	// 151 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0xc9, 0x2f, 0xc8,
	0x4c, 0x2e, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x83, 0xf0, 0x94, 0x66, 0x33, 0x72,
	0xb1, 0xf8, 0xe5, 0xa7, 0xa4, 0x0a, 0x09, 0x70, 0x31, 0x3b, 0x95, 0xa6, 0x49, 0x30, 0x29, 0x30,
	0x6a, 0xf0, 0x04, 0x81, 0x98, 0x42, 0x66, 0x5c, 0x1c, 0xce, 0x19, 0x99, 0x39, 0x29, 0x45, 0xa9,
	0x79, 0x12, 0xcc, 0x0a, 0xcc, 0x1a, 0xdc, 0x46, 0x52, 0x7a, 0x50, 0x33, 0x40, 0x3a, 0xf4, 0x60,
	0x92, 0xae, 0x79, 0x25, 0x45, 0x95, 0x41, 0x70, 0xb5, 0x52, 0x9e, 0x5c, 0xbc, 0x28, 0x52, 0x20,
	0xa3, 0xb3, 0x53, 0x2b, 0x25, 0x18, 0x15, 0x18, 0x35, 0x38, 0x83, 0x40, 0x4c, 0x21, 0x25, 0x2e,
	0xd6, 0xb2, 0xc4, 0x9c, 0xd2, 0x54, 0xb0, 0x75, 0xdc, 0x46, 0x3c, 0xc8, 0xe6, 0x06, 0x41, 0xa4,
	0xac, 0x98, 0x2c, 0x18, 0x93, 0xd8, 0xc0, 0x8e, 0x35, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0xa5,
	0xad, 0x20, 0x7a, 0xbc, 0x00, 0x00, 0x00,
}
