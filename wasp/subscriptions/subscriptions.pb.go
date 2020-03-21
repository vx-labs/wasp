// Code generated by protoc-gen-go. DO NOT EDIT.
// source: subscriptions.proto

package subscriptions

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
	Recipients           []string         `protobuf:"bytes,1,rep,name=Recipients,proto3" json:"Recipients,omitempty"`
	Qos                  []int32          `protobuf:"varint,2,rep,packed,name=Qos,proto3" json:"Qos,omitempty"`
	Children             map[string]*Node `protobuf:"bytes,3,rep,name=Children,proto3" json:"Children,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}         `json:"-"`
	XXX_unrecognized     []byte           `json:"-"`
	XXX_sizecache        int32            `json:"-"`
}

func (m *Node) Reset()         { *m = Node{} }
func (m *Node) String() string { return proto.CompactTextString(m) }
func (*Node) ProtoMessage()    {}
func (*Node) Descriptor() ([]byte, []int) {
	return fileDescriptor_18bcb28a8eb272b7, []int{0}
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

func (m *Node) GetRecipients() []string {
	if m != nil {
		return m.Recipients
	}
	return nil
}

func (m *Node) GetQos() []int32 {
	if m != nil {
		return m.Qos
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
	proto.RegisterType((*Node)(nil), "subscriptions.Node")
	proto.RegisterMapType((map[string]*Node)(nil), "subscriptions.Node.ChildrenEntry")
}

func init() { proto.RegisterFile("subscriptions.proto", fileDescriptor_18bcb28a8eb272b7) }

var fileDescriptor_18bcb28a8eb272b7 = []byte{
	// 185 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x2e, 0x2e, 0x4d, 0x2a,
	0x4e, 0x2e, 0xca, 0x2c, 0x28, 0xc9, 0xcc, 0xcf, 0x2b, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17,
	0xe2, 0x45, 0x11, 0x54, 0x3a, 0xc9, 0xc8, 0xc5, 0xe2, 0x97, 0x9f, 0x92, 0x2a, 0x24, 0xc7, 0xc5,
	0x15, 0x94, 0x9a, 0x9c, 0x59, 0x90, 0x99, 0x9a, 0x57, 0x52, 0x2c, 0xc1, 0xa8, 0xc0, 0xac, 0xc1,
	0x19, 0x84, 0x24, 0x22, 0x24, 0xc0, 0xc5, 0x1c, 0x98, 0x5f, 0x2c, 0xc1, 0xa4, 0xc0, 0xac, 0xc1,
	0x1a, 0x04, 0x62, 0x0a, 0xd9, 0x72, 0x71, 0x38, 0x67, 0x64, 0xe6, 0xa4, 0x14, 0xa5, 0xe6, 0x49,
	0x30, 0x2b, 0x30, 0x6b, 0x70, 0x1b, 0x29, 0xea, 0xa1, 0xda, 0x08, 0x32, 0x58, 0x0f, 0xa6, 0xc6,
	0x35, 0xaf, 0xa4, 0xa8, 0x32, 0x08, 0xae, 0x45, 0x2a, 0x80, 0x8b, 0x17, 0x45, 0x0a, 0x64, 0x43,
	0x76, 0x6a, 0xa5, 0x04, 0xa3, 0x02, 0xa3, 0x06, 0x67, 0x10, 0x88, 0x29, 0xa4, 0xc9, 0xc5, 0x5a,
	0x96, 0x98, 0x53, 0x9a, 0x2a, 0xc1, 0xa4, 0xc0, 0xa8, 0xc1, 0x6d, 0x24, 0x8c, 0xc5, 0xf8, 0x20,
	0x88, 0x0a, 0x2b, 0x26, 0x0b, 0xc6, 0x24, 0x36, 0xb0, 0x0f, 0x8d, 0x01, 0x01, 0x00, 0x00, 0xff,
	0xff, 0x23, 0x90, 0x5c, 0x18, 0xf8, 0x00, 0x00, 0x00,
}
